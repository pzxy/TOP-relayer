package toprelayer

import (
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/wonderivan/logger"
	"io"
	"net/http"
	"sync"
)

var relayerHttp *Eth2TopRelayerV2

type BaseData struct {
	Message string `json:"message"`
	Success bool   `json:"success"`
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte("Hello, world!")); err != nil {
		logger.Error("helloHandler:", err)
	}
}

func getClientModeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req := struct {
		Data struct {
			Mode int `json:"mode"`
		} `json:"data"`
		BaseData
	}{}
	var err error
	defer func() {
		if err != nil {
			req.Success = false
			req.Message = err.Error()
			logger.Error("getEthHeadersHandler: ", err.Error())
		} else {
			logger.Info("getClientModeHandler success")
		}
		result, _ := json.Marshal(req)
		if _, err := w.Write(result); err != nil {
			logger.Error("response getClientModeHandler:", err)
		}
	}()
	var mode uint8
	if mode, err = relayerHttp.callerSession.GetClientMode(); err != nil {
		return
	}
	req.Data.Mode = int(mode)
	req.Success = true
	req.Message = "success"
}

func getEthHeadersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	req := struct {
		Data struct {
			Headers []*types.Header `json:"headers"`
		} `json:"data"`
		BaseData
	}{}
	var err error
	defer func() {
		if err != nil {
			req.Success = false
			req.Message = err.Error()
			logger.Error("getEthHeadersHandler: ", err.Error())
		} else {
			logger.Info("getEthHeadersHandler success")
		}
		result, _ := json.Marshal(req)
		if _, err := w.Write(result); err != nil {
			logger.Error("response getEthHeadersHandler:", err)
		}
	}()
	var eth1 []*types.Header
	if eth1, err = relayerHttp.buildEthHeader(); err != nil {
		return
	}
	req.Data.Headers = eth1
	req.Success = true
	req.Message = "success"
}

func submitEthHeadersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resp := BaseData{}
	var err error
	defer func() {
		if err != nil {
			resp.Success = false
			resp.Message = err.Error()
			logger.Error("submitEthHeadersHandler: ", err.Error())
		} else {
			logger.Info("submitEthHeadersHandler success")
		}
		result, _ := json.Marshal(resp)
		if _, err := w.Write(result); err != nil {
			logger.Error("response submitEthHeadersHandler:", err)
		}
	}()
	var body []byte
	if body, err = io.ReadAll(r.Body); err != nil {
		return
	}
	req := struct {
		Data struct {
			Headers []*types.Header `json:"headers"`
		} `json:"data"`
	}{}
	if err = json.Unmarshal(body, &req); err != nil {
		return
	}
	if err = relayerHttp.submitEthHeader(req.Data.Headers); err != nil {
		return
	}
	resp.Success = true
	resp.Message = "success"
}

func submitLightClientUpdateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req BaseData
	var err error
	defer func() {
		if err != nil {
			req.Success = false
			req.Message = err.Error()
			logger.Error("submitEthHeadersHandler: ", err.Error())
		} else {
			logger.Info("submitLightClientUpdateHandler success")
		}
		result, _ := json.Marshal(req)
		if _, err := w.Write(result); err != nil {
			logger.Error("response submitEthHeadersHandler:", err)
		}
	}()
	if err = relayerHttp.sendLightClientUpdatesWithChecks(); err != nil {
		return
	}
	req.Success = true
	req.Message = "success"
}

func (relayer *Eth2TopRelayerV2) StartRelayer2(wg *sync.WaitGroup) error {
	relayerHttp = relayer
	logger.Info("Start Eth2TopRelayerV2")
	go func() {
		defer wg.Done()
		http.HandleFunc("/", helloHandler)
		http.HandleFunc("/getClientMode", getClientModeHandler)
		http.HandleFunc("/getEthHeaders", getEthHeadersHandler)
		http.HandleFunc("/submitEthHeaders", submitEthHeadersHandler)
		http.HandleFunc("/submitLightClientUpdate", submitLightClientUpdateHandler)
		fmt.Println("Server listening on port 8065...")
		if err := http.ListenAndServe(":8065", nil); err != nil {
			panic(err.Error())
		}
	}()
	return nil
}
