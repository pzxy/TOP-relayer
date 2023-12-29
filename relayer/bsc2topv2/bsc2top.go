package bsc2topv2

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
	"toprelayer/config"
	ethbridge "toprelayer/contract/top/ethclient"
	"toprelayer/wallet"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/wonderivan/logger"
)

type Bsc2TopRelayerV2 struct {
	wallet        *wallet.Wallet
	ethsdk        *ethclient.Client
	transactor    *ethbridge.EthClientTransactor
	callerSession *ethbridge.EthClientCallerSession
}

func (relayer *Bsc2TopRelayerV2) Init(cfg *config.Relayer, listenUrl []string, pass string) error {
	w, err := wallet.NewEthWallet(cfg.Url[0], listenUrl[0], cfg.KeyPath, pass)
	if err != nil {
		logger.Error("Bsc2TopRelayerV2 NewWallet error:", err)
		return err
	}
	relayer.wallet = w

	relayer.ethsdk, err = ethclient.Dial(listenUrl[0])
	if err != nil {
		logger.Error("Bsc2TopRelayerV2 ethsdk create error:", listenUrl)
		return err
	}

	topethlient, err := ethclient.Dial(cfg.Url[0])
	if err != nil {
		logger.Error("Bsc2TopRelayerV2 new topethlient error:", err)
		return err
	}
	relayer.transactor, err = ethbridge.NewEthClientTransactor(BSCClientContract, topethlient)
	if err != nil {
		logger.Error("Bsc2TopRelayerV2 NewEthClientTransactor error:", err)
		return err
	}

	relayer.callerSession = new(ethbridge.EthClientCallerSession)
	relayer.callerSession.Contract, err = ethbridge.NewEthClientCaller(BSCClientContract, topethlient)
	if err != nil {
		logger.Error("Bsc2TopRelayerV2 NewEthClientCaller error:", err)
		return err
	}
	relayer.callerSession.CallOpts = bind.CallOpts{
		Pending:     false,
		From:        relayer.wallet.Address(),
		BlockNumber: nil,
		Context:     context.Background(),
	}

	//relayer.parlia = New(relayer.ethsdk)

	return nil
}

func (et *Bsc2TopRelayerV2) submitEthHeader(header []byte) error {
	nonce, err := et.wallet.NonceAt(context.Background(), et.wallet.Address(), nil)
	if err != nil {
		logger.Error("Bsc2TopRelayerV2 NonceAt error:", err)
		return err
	}
	gaspric, err := et.wallet.SuggestGasPrice(context.Background())
	if err != nil {
		logger.Error("Bsc2TopRelayerV2 SuggestGasPrice error:", err)
		return err
	}
	packHeader, err := ethbridge.PackSyncParam(header)
	if err != nil {
		logger.Error("Bsc2TopRelayerV2 PackSyncParam error:", err)
		return err
	}
	gaslimit, err := et.wallet.EstimateGas(context.Background(), &BSCClientContract, packHeader)
	if err != nil {
		logger.Error("Bsc2TopRelayerV2 EstimateGas error:", err)
		return err
	}
	//must init ops as bellow
	ops := &bind.TransactOpts{
		From:      et.wallet.Address(),
		Nonce:     big.NewInt(0).SetUint64(nonce),
		GasLimit:  gaslimit,
		GasFeeCap: gaspric,
		GasTipCap: big.NewInt(0),
		Signer:    et.signTransaction,
		Context:   context.Background(),
		NoSend:    false,
	}
	sigTx, err := et.transactor.Sync(ops, header)
	if err != nil {
		logger.Error("Bsc2TopRelayerV2 sync error:", err)
		return err
	}

	logger.Info("Bsc2TopRelayerV2 tx info, account[%v] nonce:%v,capfee:%v,hash:%v,size:%v", et.wallet.Address(), nonce, gaspric, sigTx.Hash(), len(header))
	return nil
}

// callback function to sign tx before send.
func (et *Bsc2TopRelayerV2) signTransaction(addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
	acc := et.wallet.Address()
	if strings.EqualFold(acc.Hex(), addr.Hex()) {
		stx, err := et.wallet.SignTx(tx)
		if err != nil {
			return nil, err
		}
		return stx, nil
	}
	return nil, fmt.Errorf("TopRelayer address:%v not available", addr)
}

func (et *Bsc2TopRelayerV2) StartRelayer(wg *sync.WaitGroup) error {
	logger.Info("Bsc2TopRelayerV2 start... subBatch: %v certaintyBlocks: %v", BATCH_NUM, CONFIRM_NUM)
	defer wg.Done()

	go func() {
		var delay time.Duration = time.Duration(1)
		for {
			logger.Info("Bsc2TopRelayerV2 ===================== New Cycle Start =====================")
			time.Sleep(time.Second * delay)
			destHeight, err := et.callerSession.GetHeight()
			if err != nil {
				logger.Error("Bsc2TopRelayerV2 get top height error:", err)
				delay = time.Duration(ERRDELAY)
				continue
			}
			if destHeight == 0 {
				logger.Info("Bsc2TopRelayerV2 not init yet")
				delay = time.Duration(ERRDELAY)
				continue
			}
			srcHeight, err := et.ethsdk.BlockNumber(context.Background())
			if err != nil {
				logger.Error("Bsc2TopRelayerV2 get bsc number error:", err)
				delay = time.Duration(ERRDELAY)
				continue
			}
			logger.Info("Bsc2TopRelayerV2 current height, top:(%s),bsc:(%s)", destHeight, srcHeight)
			if destHeight+1+CONFIRM_NUM > srcHeight {
				logger.Debug("Bsc2TopRelayerV2 waiting bsc height update")
				delay = time.Duration(WAITDELAY)
				continue
			}

			// check fork
			//checkError := false
			//for {
			//	header, err := et.ethsdk.HeaderByNumber(context.Background(), big.NewInt(0).SetUint64(destHeight))
			//	if err != nil {
			//		logger.Error("Bsc2TopRelayerV2 HeaderByNumber error:", err)
			//		checkError = true
			//		break
			//	}
			//	// get known hashes with destHeight, mock now
			//	isKnown, err := et.callerSession.IsKnown(header.Number, header.Hash())
			//	if err != nil {
			//		logger.Error("Bsc2TopRelayerV2 IsKnown error:", err)
			//		checkError = true
			//		break
			//	}
			//	if isKnown {
			//		logger.Debug("%v hash is known", header.Number)
			//		break
			//	} else {
			//		logger.Warn("%v hash is not known", header.Number)
			//		destHeight -= 1
			//	}
			//}
			//
			//if checkError {
			//	delay = time.Duration(ERRDELAY)
			//	break
			//}
			//
			syncStartHeight := destHeight + 1
			syncNum := srcHeight - CONFIRM_NUM - destHeight
			if syncNum > BATCH_NUM {
				syncNum = BATCH_NUM
			}
			syncEndHeight := syncStartHeight + syncNum - 1
			logger.Info("Bsc2TopRelayerV2 sync from (%v) to (%v)", syncStartHeight, syncEndHeight)
			err = et.signAndSendTransactions(syncStartHeight, syncEndHeight)
			if err != nil {
				logger.Error("Bsc2TopRelayerV2 signAndSendTransactions failed:", err)
				delay = time.Duration(ERRDELAY)
				break
			}
			logger.Info("Bsc2TopRelayerV2 sync round finish")
			if syncNum == BATCH_NUM {
				delay = time.Duration(SUCCESSDELAY)
			} else {
				delay = time.Duration(WAITDELAY)
			}
			// break
		}
	}()

	logger.Error("Bsc2TopRelayerV2 timeout")
	return nil
}

func (et *Bsc2TopRelayerV2) signAndSendTransactions(lo, hi uint64) error {
	var batch []byte
	for h := lo; h <= hi; h++ {
		header, err := et.ethsdk.HeaderByNumber(context.Background(), big.NewInt(0).SetUint64(h))
		if err != nil {
			logger.Error(err)
			break
		}
		rlpHeader, err := rlp.EncodeToBytes(header)
		if err != nil {
			logger.Error(err)
			break
		}
		batch = append(batch, rlpHeader...)
	}
	if len(batch) > 0 {
		err := et.submitEthHeader(batch)
		if err != nil {
			logger.Error("Bsc2TopRelayerV2 submitHeaders failed:", err)
			return err
		}
	}
	return nil
}

func (et *Bsc2TopRelayerV2) GetInitData() ([]byte, error) {
	destHeight, err := et.callerSession.GetHeight()
	if err != nil {
		logger.Error("Bsc2TopRelayerV2 get height error:", err)
		return nil, err
	}
	height := (destHeight - 11) / 200 * 200
	logger.Error("heco init with height: %v - %v", height, height+11)
	var batch []byte
	for i := height; i <= height+11; i++ {
		header, err := et.ethsdk.HeaderByNumber(context.Background(), big.NewInt(0).SetUint64(i))
		if err != nil {
			logger.Error("Bsc2TopRelayerV2 HeaderByNumber error:", err)
			return nil, err
		}
		rlp_bytes, err := rlp.EncodeToBytes(header)
		if err != nil {
			logger.Error("Bsc2TopRelayerV2 EncodeToBytes error:", err)
			return nil, err
		}
		batch = append(batch, rlp_bytes...)
	}

	return batch, nil
}