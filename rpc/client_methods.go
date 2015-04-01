package rpc

import (
	"fmt"
	"github.com/tendermint/tendermint2/account"
	"github.com/tendermint/tendermint2/binary"
	"github.com/tendermint/tendermint2/rpc/core"
	"github.com/tendermint/tendermint2/types"
	"io/ioutil"
	"net/http"
)

type Client interface {
	BlockchainInfo(minHeight uint) (*core.ResponseBlockchainInfo, error)
	BroadcastTx(tx types.Tx) (*core.ResponseBroadcastTx, error)
	Call(address []byte) (*core.ResponseCall, error)
	DumpStorage(addr []byte) (*core.ResponseDumpStorage, error)
	GenPrivAccount() (*core.ResponseGenPrivAccount, error)
	GetAccount(address []byte) (*core.ResponseGetAccount, error)
	GetBlock(height uint) (*core.ResponseGetBlock, error)
	GetStorage(address []byte) (*core.ResponseGetStorage, error)
	ListAccounts() (*core.ResponseListAccounts, error)
	ListValidators() (*core.ResponseListValidators, error)
	NetInfo() (*core.ResponseNetInfo, error)
	SignTx(tx types.Tx, privAccounts []*account.PrivAccount) (*core.ResponseSignTx, error)
	Status() (*core.ResponseStatus, error)
}

func (c *ClientHTTP) BlockchainInfo(minHeight uint) (*core.ResponseBlockchainInfo, error) {
	values, err := argsToURLValues([]string{"minHeight"}, minHeight)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"blockchain_info", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseBlockchainInfo `json:"result"`
		Error   string                       `json:"error"`
		Id      string                       `json:"id"`
		JSONRPC string                       `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) BroadcastTx(tx types.Tx) (*core.ResponseBroadcastTx, error) {
	values, err := argsToURLValues([]string{"tx"}, tx)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"broadcast_tx", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseBroadcastTx `json:"result"`
		Error   string                    `json:"error"`
		Id      string                    `json:"id"`
		JSONRPC string                    `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) Call(address []byte) (*core.ResponseCall, error) {
	values, err := argsToURLValues([]string{"address"}, address)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"call", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseCall `json:"result"`
		Error   string             `json:"error"`
		Id      string             `json:"id"`
		JSONRPC string             `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) DumpStorage(addr []byte) (*core.ResponseDumpStorage, error) {
	values, err := argsToURLValues([]string{"addr"}, addr)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"dump_storage", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseDumpStorage `json:"result"`
		Error   string                    `json:"error"`
		Id      string                    `json:"id"`
		JSONRPC string                    `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) GenPrivAccount() (*core.ResponseGenPrivAccount, error) {
	values, err := argsToURLValues(nil, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"gen_priv_account", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseGenPrivAccount `json:"result"`
		Error   string                       `json:"error"`
		Id      string                       `json:"id"`
		JSONRPC string                       `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) GetAccount(address []byte) (*core.ResponseGetAccount, error) {
	values, err := argsToURLValues([]string{"address"}, address)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"get_account", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseGetAccount `json:"result"`
		Error   string                   `json:"error"`
		Id      string                   `json:"id"`
		JSONRPC string                   `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) GetBlock(height uint) (*core.ResponseGetBlock, error) {
	values, err := argsToURLValues([]string{"height"}, height)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"get_block", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseGetBlock `json:"result"`
		Error   string                 `json:"error"`
		Id      string                 `json:"id"`
		JSONRPC string                 `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) GetStorage(address []byte) (*core.ResponseGetStorage, error) {
	values, err := argsToURLValues([]string{"address"}, address)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"get_storage", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseGetStorage `json:"result"`
		Error   string                   `json:"error"`
		Id      string                   `json:"id"`
		JSONRPC string                   `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) ListAccounts() (*core.ResponseListAccounts, error) {
	values, err := argsToURLValues(nil, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"list_accounts", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseListAccounts `json:"result"`
		Error   string                     `json:"error"`
		Id      string                     `json:"id"`
		JSONRPC string                     `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) ListValidators() (*core.ResponseListValidators, error) {
	values, err := argsToURLValues(nil, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"list_validators", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseListValidators `json:"result"`
		Error   string                       `json:"error"`
		Id      string                       `json:"id"`
		JSONRPC string                       `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) NetInfo() (*core.ResponseNetInfo, error) {
	values, err := argsToURLValues(nil, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"net_info", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseNetInfo `json:"result"`
		Error   string                `json:"error"`
		Id      string                `json:"id"`
		JSONRPC string                `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) SignTx(tx types.Tx, privAccounts []*account.PrivAccount) (*core.ResponseSignTx, error) {
	values, err := argsToURLValues([]string{"tx", "privAccounts"}, tx, privAccounts)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"sign_tx", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseSignTx `json:"result"`
		Error   string               `json:"error"`
		Id      string               `json:"id"`
		JSONRPC string               `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientHTTP) Status() (*core.ResponseStatus, error) {
	values, err := argsToURLValues(nil, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.PostForm(c.addr+"status", values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseStatus `json:"result"`
		Error   string               `json:"error"`
		Id      string               `json:"id"`
		JSONRPC string               `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) BlockchainInfo(minHeight uint) (*core.ResponseBlockchainInfo, error) {
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  "blockchain_info",
		Params:  []interface{}{minHeight},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseBlockchainInfo `json:"result"`
		Error   string                       `json:"error"`
		Id      string                       `json:"id"`
		JSONRPC string                       `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) BroadcastTx(tx types.Tx) (*core.ResponseBroadcastTx, error) {
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  "broadcast_tx",
		Params:  []interface{}{tx},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseBroadcastTx `json:"result"`
		Error   string                    `json:"error"`
		Id      string                    `json:"id"`
		JSONRPC string                    `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) Call(address []byte) (*core.ResponseCall, error) {
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  "call",
		Params:  []interface{}{address},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseCall `json:"result"`
		Error   string             `json:"error"`
		Id      string             `json:"id"`
		JSONRPC string             `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) DumpStorage(addr []byte) (*core.ResponseDumpStorage, error) {
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  "dump_storage",
		Params:  []interface{}{addr},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseDumpStorage `json:"result"`
		Error   string                    `json:"error"`
		Id      string                    `json:"id"`
		JSONRPC string                    `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) GenPrivAccount() (*core.ResponseGenPrivAccount, error) {
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  "gen_priv_account",
		Params:  []interface{}{nil},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseGenPrivAccount `json:"result"`
		Error   string                       `json:"error"`
		Id      string                       `json:"id"`
		JSONRPC string                       `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) GetAccount(address []byte) (*core.ResponseGetAccount, error) {
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  "get_account",
		Params:  []interface{}{address},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseGetAccount `json:"result"`
		Error   string                   `json:"error"`
		Id      string                   `json:"id"`
		JSONRPC string                   `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) GetBlock(height uint) (*core.ResponseGetBlock, error) {
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  "get_block",
		Params:  []interface{}{height},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseGetBlock `json:"result"`
		Error   string                 `json:"error"`
		Id      string                 `json:"id"`
		JSONRPC string                 `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) GetStorage(address []byte) (*core.ResponseGetStorage, error) {
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  "get_storage",
		Params:  []interface{}{address},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseGetStorage `json:"result"`
		Error   string                   `json:"error"`
		Id      string                   `json:"id"`
		JSONRPC string                   `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) ListAccounts() (*core.ResponseListAccounts, error) {
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  "list_accounts",
		Params:  []interface{}{nil},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseListAccounts `json:"result"`
		Error   string                     `json:"error"`
		Id      string                     `json:"id"`
		JSONRPC string                     `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) ListValidators() (*core.ResponseListValidators, error) {
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  "list_validators",
		Params:  []interface{}{nil},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseListValidators `json:"result"`
		Error   string                       `json:"error"`
		Id      string                       `json:"id"`
		JSONRPC string                       `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) NetInfo() (*core.ResponseNetInfo, error) {
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  "net_info",
		Params:  []interface{}{nil},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseNetInfo `json:"result"`
		Error   string                `json:"error"`
		Id      string                `json:"id"`
		JSONRPC string                `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) SignTx(tx types.Tx, privAccounts []*account.PrivAccount) (*core.ResponseSignTx, error) {
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  "sign_tx",
		Params:  []interface{}{tx, privAccounts},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseSignTx `json:"result"`
		Error   string               `json:"error"`
		Id      string               `json:"id"`
		JSONRPC string               `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}

func (c *ClientJSON) Status() (*core.ResponseStatus, error) {
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  "status",
		Params:  []interface{}{nil},
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result  *core.ResponseStatus `json:"result"`
		Error   string               `json:"error"`
		Id      string               `json:"id"`
		JSONRPC string               `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}
