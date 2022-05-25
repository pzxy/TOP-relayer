package top2eth

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
	"toprelayer/contract/eth/hsc"
	"toprelayer/msg"
	"toprelayer/sdk/ethsdk"
	"toprelayer/sdk/topsdk"
	"toprelayer/util"
	"toprelayer/wallet"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/wonderivan/logger"
)

const (
	METHOD_GETCURRENTBLOCKHEIGHT        = "getCurrentBlockHeight"
	CONFIRMSUCCESS               string = "0x1"

	SUCCESSDELAY int64 = 15 //mainnet 1000
	FATALTIMEOUT int64 = 24 //hours
	FORKDELAY    int64 = 5  //mainnet 3000 seconds
	ERRDELAY     int64 = 10
)

type Top2EthRelayer struct {
	context.Context
	contract        common.Address
	chainId         uint64
	wallet          wallet.IWallet
	ethsdk          *ethsdk.EthSdk
	topsdk          *topsdk.TopSdk
	certaintyBlocks int
	subBatch        int
}

func (te *Top2EthRelayer) Init(ethUrl, topUrl, keypath, pass string, chainid uint64, contract common.Address, batch, cert int, verify bool) error {
	ethsdk, err := ethsdk.NewEthSdk(ethUrl)
	if err != nil {
		return err
	}
	topsdk, err := topsdk.NewTopSdk(topUrl)
	if err != nil {
		return err
	}
	te.topsdk = topsdk
	te.ethsdk = ethsdk
	te.contract = contract
	te.chainId = chainid
	te.subBatch = batch
	te.certaintyBlocks = cert

	w, err := wallet.NewWallet(ethUrl, keypath, pass, chainid)
	if err != nil {
		return err
	}
	te.wallet = w
	return nil
}

func (te *Top2EthRelayer) ChainId() uint64 {
	return te.chainId
}

func (te *Top2EthRelayer) submitTopHeader(headers []byte, nonce uint64) (*types.Transaction, error) {
	logger.Info("submitTopHeader length:%v,chainid:%v", len(headers), te.chainId)
	gaspric, err := te.wallet.GasPrice(context.Background())
	if err != nil {
		return nil, err
	}
	/* msg := ethereum.CallMsg{
		From:     te.wallet.CurrentAccount().Address,
		To:       &te.contract,
		GasPrice: gaspric,
		Value:    big.NewInt(0),
		Data:     headers,
	}

	gaslimit, err := te.wallet.EstimateGas(context.Background(), msg)
	if err != nil {
		fmt.Println("EstimateGas error:", err)
		return nil, err
	} */

	//test mock
	gaslimit := uint64(500000)

	balance, err := te.wallet.GetBalance(te.wallet.CurrentAccount().Address)
	if err != nil {
		return nil, err
	}
	logger.Info("account[%v] balance:%v,nonce:%v,gasprice:%v,gaslimit:%v", te.wallet.CurrentAccount().Address, balance.Uint64(), nonce, gaspric.Uint64(), gaslimit)
	if balance.Uint64() <= gaspric.Uint64()*gaslimit {
		return nil, fmt.Errorf("account[%v] not sufficient funds,balance:%v", te.wallet.CurrentAccount().Address, balance.Uint64())
	}

	//must init ops as bellow
	ops := &bind.TransactOpts{
		From:     te.wallet.CurrentAccount().Address,
		Nonce:    big.NewInt(0).SetUint64(nonce),
		GasPrice: gaspric,
		GasLimit: gaslimit,
		Signer:   te.signTransaction,
		Context:  context.Background(),
		NoSend:   true,
	}

	contractcaller, err := hsc.NewHscTransactor(te.contract, te.ethsdk)
	if err != nil {
		logger.Error("Top2EthRelayer NewBridgeTransactor:", err)
		return nil, err
	}

	sigTx, err := contractcaller.SyncBlockHeader(ops, headers)
	if err != nil {
		logger.Error("Top2EthRelayer AddLightClientBlock error:", err)
		return nil, err
	}

	if ops.NoSend {
		err = util.VerifyEthSignature(sigTx)
		if err != nil {
			logger.Error("Top2EthRelayer VerifyEthSignature error:", err)
			return nil, err
		}

		err := te.ethsdk.SendTransaction(ops.Context, sigTx)
		if err != nil {
			logger.Error("Top2EthRelayer SendTransaction error:", err)
			return nil, err
		}
	}
	logger.Debug("hash:%v", sigTx.Hash())
	return sigTx, nil
}

//callback function to sign tx before send.
func (te *Top2EthRelayer) signTransaction(addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
	acc := te.wallet.CurrentAccount()
	if strings.EqualFold(acc.Address.Hex(), addr.Hex()) {
		stx, err := te.wallet.SignTx(tx)
		if err != nil {
			return nil, err
		}
		return stx, nil
	}
	return nil, fmt.Errorf("address:%v not available", addr)
}

func (te *Top2EthRelayer) getEthBridgeState() (*msg.BridgeState, error) {
	hscaller, err := hsc.NewHscCaller(te.contract, te.ethsdk)
	if err != nil {
		return nil, err
	}

	hscRaw := hsc.HscCallerRaw{Contract: hscaller}
	result := make([]interface{}, 1)
	err = hscRaw.Call(nil, &result, METHOD_GETCURRENTBLOCKHEIGHT, te.chainId)
	if err != nil {
		return nil, err
	}

	state, success := result[0].(msg.BridgeState)
	if !success {
		return nil, err
	}

	return &state, nil
}

func (te *Top2EthRelayer) StartRelayer(wg *sync.WaitGroup) error {
	logger.Info("Start Top2EthRelayer relayer... chainid:%v", te.chainId)
	defer wg.Done()

	timeoutDur := time.Duration(time.Second * 300) //test mock
	//timeoutDur := time.Duration(time.Hour * FATALTIMEOUT)
	timeout := time.NewTimer(timeoutDur)
	defer timeout.Stop()

	go func(timeoutDur time.Duration, timeout *time.Timer) {
		var syncStartHeight uint64 = 1
		//test mock
		var topConfirmedBlockHeight uint64 = 1000
		var delay time.Duration = time.Duration(1)

		for {
			time.Sleep(time.Second * delay)
			/* bridgeState, err := te.getEthBridgeState()
			if err != nil {
				logger.Error(err)
				delay = time.Duration(ERRDELAY)
				continue
			}

			if bridgeState.ConfirmState == CONFIRMSUCCESS {
				syncStartHeight = bridgeState.LatestSyncedHeight.Uint64() + 1
			} else {
				logger.Warn("eth bridge confirm top header failed,height:%v.", bridgeState.LatestConfirmedHeight.Uint64()+1)
				syncStartHeight = bridgeState.LatestConfirmedHeight.Uint64()
			}

			topCurrentHeight, err := te.topsdk.GetLatestTopElectBlockHeight()
			if err != nil {
				logger.Error(err)
				delay = time.Duration(ERRDELAY)
				continue
			}
			topConfirmedBlockHeight := topCurrentHeight - 2 - uint64(te.certaintyBlocks)
			*/

			//if syncStartHeight <= topConfirmedBlockHeight {
			hashes, err := te.signAndSendTransactions(syncStartHeight, topConfirmedBlockHeight)
			if len(hashes) > 0 {
				logger.Info("Top2EthRelayer sent block header from %v to :%v", syncStartHeight, topConfirmedBlockHeight)
				delay = time.Duration(SUCCESSDELAY * int64(len(hashes)))
				timeout.Reset(timeoutDur)
				logger.Debug("timeout.Reset:%v", timeoutDur)
				syncStartHeight = topConfirmedBlockHeight + 1
				topConfirmedBlockHeight += 500 //test mock
				continue
			}
			if err != nil {
				logger.Error("Top2EthRelayer signAndSendTransactions failed:%v", err)
				delay = time.Duration(ERRDELAY)
				continue
			}
			// }
			//top fork?
			//logger.Error("eth chain revert? syncStartHeight[%v] > topConfirmedBlockHeight[%v]", syncStartHeight, topConfirmedBlockHeight)
			//delay = time.Duration(FORKDELAY)
		}
	}(timeoutDur, timeout)

	<-timeout.C
	logger.Error("relayer [%v] timeout.", te.chainId)
	return nil
}

func (te *Top2EthRelayer) batch(headers []*msg.TopElectBlockHeader, nonce uint64) (common.Hash, error) {
	logger.Info("batch headers number:", len(headers))
	data, err := msg.EncodeHeaders(headers)
	if err != nil {
		logger.Error("Top2EthRelayer EncodeHeaders failed:", err)
		return common.Hash{}, err
	}

	tx, err := te.submitTopHeader(data, nonce)
	if err != nil {
		logger.Error("Top2EthRelayer submitHeaders failed:", err)
		return common.Hash{}, err
	}
	return tx.Hash(), nil
}

func (te *Top2EthRelayer) signAndSendTransactions(lo, hi uint64) ([]common.Hash, error) {
	logger.Info("signAndSendTransactions height form:%v,to%v", lo, hi)
	var batchHeaders []*msg.TopElectBlockHeader
	var hashes []common.Hash
	nonce, err := te.wallet.GetNonce(te.wallet.CurrentAccount().Address)
	if err != nil {
		return hashes, err
	}

	h := lo
	for ; h <= hi; h++ {
		/* header, err := te.topsdk.GetTopElectBlockHeadByHeight(h, topsdk.ElectBlock_Current)
		if err != nil {
			logger.Error(err)
			return hashes, err
		} */
		//test mock
		header := &msg.TopElectBlockHeader{BlockNumber: uint64(h)}

		batchHeaders = append(batchHeaders, header)
		if (h-lo+1)%uint64(te.subBatch) == 0 {
			hash, err := te.batch(batchHeaders, nonce)
			if err != nil {
				return hashes, err
			}
			batchHeaders = []*msg.TopElectBlockHeader{}
			hashes = append(hashes, hash)
			nonce++
		}
	}
	if h > hi {
		if len(batchHeaders) > 0 {
			hash, err := te.batch(batchHeaders, nonce)
			if err != nil {
				return hashes, err
			}
			batchHeaders = []*msg.TopElectBlockHeader{}
			hashes = append(hashes, hash)
		}
	}

	return hashes, nil
}
