package main

//
// rpc helper functions
//

import "encoding/json"
import "errors"
import "math/big"
import "net/http"
import "strings"
import "bufio"
import "io/ioutil"

type RPCTarget interface {
	SendRPCRequest(request RPCRequest) (*RPCResponse, error)
}

type RPCParams []interface{}

type RPCRequest struct {
	Id      interface{} `json:"id"`
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  RPCParams   `json:"params"`
}

func (self *RPCRequest) ToJson() string {
	bytes, _ := json.Marshal(self)
	return string(bytes)
}

func (self *RPCRequest) GetParam(i int) (string, error) {
	if len(self.Params) < i {
		return "", errors.New("parameter index out of range")
	}

	str, ok := self.Params[i].(string)

	if !ok {
		return "", errors.New("could not convert parameter to string")
	}

	return str, nil
}

func (self *RPCRequest) GetBigIntParam(i, size int) (*big.Int, error) {
	str, err := self.GetParam(i)

	if err != nil {
		return nil, err
	}

    if strings.HasPrefix(str, "0x") {
	    return parseHex(str, size)
    } else {
        ret := big.NewInt(0)
        ret.SetString(str, 10)
        return ret, nil
    }
}

func (self *RPCRequest) ReplaceParam(i int, str string) error {
	if len(self.Params) < i {
		return errors.New("parameter index out of range")
	}

	self.Params[i] = str

	return nil
}

type RPCError struct {
	Code    float64       `json:"code"`
	Message string        `json:"message"`
	Data    *RPCErrorData `json:data,omitempty"`
}

type RPCResult interface{}
type RPCResultArray []interface{}
type RPCErrorData interface{}
type RPCId interface{}

type RPCResponse struct {
	Id      RPCId      `json:"id"`
	Jsonrpc string     `json:"jsonrpc"`
	Result  *RPCResult `json:"result,omitempty"`
	Error   *RPCError  `json:"error,omitempty"`
}

func (self *RPCResponse) ToJson() string {
	bytes, _ := json.Marshal(self)
	return string(bytes)
}

func (self *RPCResponse) GetBigIntEntryResult(i, size int) (*big.Int, error) {
	str, err := self.GetStringEntryResult(i)

	if err != nil {
		return nil, errors.New("could not get result: " + err.Error())
	}

	return parseHex(str, size)
}

func (self *RPCResponse) GetStringEntryResult(i int) (string, error) {
	if self.Result == nil {
		return "", errors.New("invalid result object")
	}

	result, ok := (*self.Result).([]interface{})

	if !ok {
		return "", errors.New("result object is not array")
	}

	if len(result) < i {
		return "", errors.New("index out of bounds")
	}

	str, ok := result[i].(string)

	if !ok {
		return "", errors.New("result index is not a string")
	}

	return str, nil
}

func (self *RPCResponse) GetStringResult() (string, error) {
	if self.Result == nil {
		return "", errors.New("could not get string result in response")
	}

	ret, ok := (*self.Result).(string)

	if !ok {
		return "", errors.New("could not convert result to string")
	}

	return ret, nil
}

func (self *RPCResponse) GetBigIntResult(size int) (*big.Int, error) {
	str, err := self.GetStringResult()

	if err != nil {
		return nil, errors.New("could not get result in response")
	}

	return parseHex(str, size)
}

func (self *RPCResponse) GetBoolResult() (bool, error) {
	if self.Result == nil {
		return false, errors.New("could not get string result in response")
	}

	ret, ok := (*self.Result).(bool)

	if !ok {
		return false, errors.New("could not convert result to bool")
	}

	return ret, nil
}

func (self *RPCResponse) ReplaceResult(i int, str string) error {
	if self.Result == nil {
		return errors.New("could not get result in response")
	}

	result, ok := (*self.Result).([]interface{})

	if !ok {
		return errors.New("results are not an array")
	}

	if len(result) < i {
		return errors.New("result index out of range")
	}

	(*self.Result).([]interface{})[i] = str

	return nil
}

func NewRPCRequest(id RPCId, method string, params RPCParams) *RPCRequest {
	return &RPCRequest{Id: id, Jsonrpc: "2.0", Method: method, Params: params}
}

func NewRPCError(id RPCId, code float64, message string, data *RPCErrorData) *RPCResponse {
	err := &RPCError{Code: code, Message: message, Data: data}
	return &RPCResponse{Id: id, Jsonrpc: "2.0", Result: nil, Error: err}
}

func NewRPCResult(id RPCId, result RPCResult) *RPCResponse {
	return &RPCResponse{Id: id, Jsonrpc: "2.0", Result: &result}
}

func sendRPCRequestRaw(request *RPCRequest, dest string) ([]byte, error) {
	req, _ := http.NewRequest("POST", dest, strings.NewReader(request.ToJson()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "close")
	req.Close = true
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return nil, errors.New("server failed to communicate to ethereum network")
	}

	bytes, err := ioutil.ReadAll(bufio.NewReader(resp.Body))
	resp.Body.Close()

	if err != nil {
		return nil, errors.New("unable to unpack json response - " + err.Error())
	}

	return bytes, nil
}

func sendRPCRequest(request *RPCRequest, dest string) (*RPCResponse, error) {
	req, _ := http.NewRequest("POST", dest, strings.NewReader(request.ToJson()))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Connection", "close")
	req.Close = true
	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return nil, errors.New("server failed to communicate to ethereum network")
	}

	bytes, err := ioutil.ReadAll(bufio.NewReader(resp.Body))
	response := &RPCResponse{}
	json.Unmarshal(bytes, response)
	resp.Body.Close()

	if err != nil {
		return nil, errors.New("unable to unpack json response - " + err.Error())
	}

	return response, nil
}
