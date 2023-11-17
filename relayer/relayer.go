package relayer

import (
	"errors"
	"fmt"
	"sync"
	"toprelayer/relayer/bsc2top"
	"toprelayer/relayer/eth2top"
	"toprelayer/relayer/heco2top"
	"toprelayer/relayer/openalliance"

	"github.com/wonderivan/logger"
	"toprelayer/config"
	"toprelayer/relayer/monitor"
	"toprelayer/relayer/top2eth"
)

var (
	topRelayers = map[string]IChainRelayer{
		config.ETH_CHAIN:     new(eth2top.Eth2TopRelayerV2),
		config.BSC_CHAIN:     new(bsc2top.Bsc2TopRelayer),
		config.HECO_CHAIN:    new(heco2top.Heco2TopRelayer),
		config.OPEN_ALLIANCE: new(openalliance.OpenAlliance2TopRelayer)}

	crossChainRelayer = new(top2eth.CrossChainRelayer)
)

type IChainRelayer interface {
	Init(cfg *config.Relayer, listenUrl []string, pass string) error
	StartRelayer(*sync.WaitGroup) error
	GetInitData() ([]byte, error)
}

type ICrossChainRelayer interface {
	Init(chainName string, cfg *config.Relayer, listenUrl string, pass string) error
	StartRelayer(*sync.WaitGroup) error
}

func startTopRelayer(relayer IChainRelayer, cfg *config.Relayer, listenUrl []string, pass string, wg *sync.WaitGroup) error {
	err := relayer.Init(cfg, listenUrl, pass)
	if err != nil {
		logger.Error("startTopRelayer error:", err)
		return err
	}

	wg.Add(1)
	go func() {
		err = relayer.StartRelayer(wg)
	}()
	if err != nil {
		logger.Error("relayer.StartRelayer error:", err)
		return err
	}
	return nil
}

func startCrossChainRelayer(relayer ICrossChainRelayer, chainName string, cfg *config.Relayer, listenUrl string, pass string, wg *sync.WaitGroup) error {
	err := relayer.Init(chainName, cfg, listenUrl, pass)
	if err != nil {
		logger.Error("startCrossChainRelayer error:", err)
		return err
	}

	wg.Add(1)
	go func() {
		err = relayer.StartRelayer(wg)
	}()
	if err != nil {
		logger.Error("relayer.StartRelayer error:", err)
		return err
	}
	return nil
}

func StartRelayer(cfg *config.Config, pass string, wg *sync.WaitGroup) error {
	// start monitor
	if err := monitor.MonitorMsgInit(cfg.RelayerToRun); err != nil {
		logger.Error("MonitorMsgInit fail:", err)
		return err
	}
	// start relayer
	topConfig, exist := cfg.RelayerConfig[config.TOP_CHAIN]
	if !exist {
		return fmt.Errorf("not found TOP chain config")
	}
	RelayerConfig, exist := cfg.RelayerConfig[cfg.RelayerToRun]
	if !exist {
		return fmt.Errorf("not found config of RelayerToRun")
	}
	switch cfg.RelayerToRun {
	case config.TOP_CHAIN:
		for name, c := range cfg.RelayerConfig {
			logger.Info("name: ", name)
			if name == config.TOP_CHAIN {
				continue
			}
			if name != config.ETH_CHAIN && name != config.BSC_CHAIN && name != config.HECO_CHAIN && name != config.OPEN_ALLIANCE {
				logger.Warn("TopRelayer not support:", name)
				continue
			}
			topRelayer, exist := topRelayers[name]
			if !exist {
				logger.Warn("unknown chain config:", name)
				continue
			}
			if err := startTopRelayer(topRelayer, topConfig, c.Url, pass, wg); err != nil {
				logger.Error("StartRelayer %v error: %v", name, err)
				continue
			}
		}
	case config.HECO_CHAIN, config.BSC_CHAIN, config.ETH_CHAIN, config.OPEN_ALLIANCE:
		err := startCrossChainRelayer(crossChainRelayer, cfg.RelayerToRun, RelayerConfig, topConfig.Url[0], pass, wg)
		if err != nil {
			logger.Error("StartRelayer error:", err)
			return err
		}
	default:
		err := fmt.Errorf("Invalid RelayerToRun(%s)", cfg.RelayerToRun)
		return err
	}
	return nil
}

func GetInitData(cfg *config.Config, pass, chainName string) ([]byte, error) {
	if cfg.RelayerToRun != config.TOP_CHAIN {
		err := errors.New("RelayerToRun error")
		logger.Error(err)
		return nil, err
	}
	if chainName != config.ETH_CHAIN && chainName != config.BSC_CHAIN && chainName != config.HECO_CHAIN {
		err := errors.New("chain not support init data")
		logger.Error(err)
		return nil, err
	}
	c, exist := cfg.RelayerConfig[chainName]
	if !exist {
		err := errors.New("not found chain config")
		logger.Error(err)
		return nil, err
	}
	topRelayer, exist := topRelayers[chainName]
	if !exist {
		err := errors.New("not found chain relayer")
		logger.Error(err)
		return nil, err
	}
	err := topRelayer.Init(c, c.Url, pass)
	if err != nil {
		logger.Error("Init error:", err)
		return nil, err
	}
	return topRelayer.GetInitData()
}
