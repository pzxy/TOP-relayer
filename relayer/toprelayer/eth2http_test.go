package toprelayer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/core/types"
	"io"
	"net/http"
	"testing"
)

type HttpData2 struct {
	Action string `json:"action"`
}

const url = "http://127.0.0.1:8065"

//		http.HandleFunc("/", helloHandler)
//		http.HandleFunc("/getClientMode", getClientModeHandler)
//		http.HandleFunc("/getEthHeaders", getEthHeadersHandler)
//		http.HandleFunc("/submitEthHeaders", submitEthHeadersHandler)
//		http.HandleFunc("/submitLightClientUpdate", submitLightClientUpdateHandler)

func TestHttpGetClientMode(t *testing.T) {
	resp, err := http.Get(url + "/getClientMode")
	if err != nil {
		t.Error(err)
	}
	var body []byte
	if body, err = io.ReadAll(resp.Body); err != nil {
		t.Error(err)
		return
	}
	data := struct {
		Data struct {
			Mode int `json:"mode"`
		} `json:"data"`
		BaseData
	}{}
	if err = json.Unmarshal(body, &data); err != nil {
		t.Error(err)
		return
	}
	if !data.Success {
		t.Fatal(data.Message)
		return
	}
	t.Log("clientMode:", string(body))
}

func httpGetEthHeaders() ([]*types.Header, error) {
	resp, err := http.Get(url + "/getEthHeaders")
	if err != nil {
	}
	var body []byte
	if body, err = io.ReadAll(resp.Body); err != nil {
		return nil, err
	}
	data := struct {
		Data struct {
			Headers []*types.Header `json:"headers"`
		} `json:"data"`
		BaseData
	}{}
	if err = json.Unmarshal(body, &data); err != nil {
		return nil, err
	}
	if !data.Success {
		return nil, fmt.Errorf(data.Message)
	}
	//fmt.Println("headers:", string(body))
	return data.Data.Headers, nil
}

func TestHttpGetEthHeaders(t *testing.T) {
	headers, err := httpGetEthHeaders()
	if err != nil {
		t.Error(err)
		return
	}
	t.Log("headers:", len(headers))
}

func TestHttpSubmitEthHeaders(t *testing.T) {
	headers, err := httpGetEthHeaders()
	if err != nil {
		t.Error(err)
		return
	}
	req := struct {
		Data struct {
			Headers []*types.Header `json:"headers"`
		} `json:"data"`
	}{}
	req.Data.Headers = headers
	reqData, err := json.Marshal(req)
	if err != nil {
		t.Error(err)
		return
	}
	if resp, err := http.Post(url+"/submitEthHeaders", "application/json", bytes.NewReader(reqData)); err != nil {
		t.Error(err)
		return
	} else {
		var body []byte
		if body, err = io.ReadAll(resp.Body); err != nil {
			t.Error(err)
			return
		}
		fmt.Println("submitEthHeaders:", string(body))
		var respData BaseData
		if err = json.Unmarshal(body, &respData); err != nil {
			t.Error(err)
			return
		}
		if !respData.Success {
			t.Fatal(respData.Message)
			return
		}
	}
}

func TestHttpSubmitLightClientUpdate(t *testing.T) {
	req := BaseData{}
	reqData, err := json.Marshal(req)
	if err != nil {
		t.Error(err)
		return
	}
	if resp, err := http.Post(url+"/submitLightClientUpdate", "application/json", bytes.NewReader(reqData)); err != nil {
		t.Error(err)
		return
	} else {
		var body []byte
		if body, err = io.ReadAll(resp.Body); err != nil {
			t.Error(err)
			return
		}
		fmt.Println(string(body))
		var respData BaseData
		if err = json.Unmarshal(body, &respData); err != nil {
			t.Error(err)
			return
		}
		if !respData.Success {
			t.Fatal(respData.Message)
			return
		}
	}
}
