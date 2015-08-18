package core_client

import (
	"bytes"
	"fmt"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpctypes "github.com/tendermint/tendermint/rpc/types"
	"github.com/tendermint/tendermint/wire"
	"io/ioutil"
	"net/http"
	"net/url"

	//"reflect"
	// Uncomment to use go:generate
	// _ "github.com/tendermint/go-rpc-gen"
)

// maps camel-case function names to lower case rpc version
var reverseFuncMap = map[string]string{
	"Status":             "status",
	"NetInfo":            "net_info",
	"BlockchainInfo":     "blockchain",
	"Genesis":            "genesis",
	"GetBlock":           "get_block",
	"GetAccount":         "get_account",
	"GetStorage":         "get_storage",
	"Call":               "call",
	"CallCode":           "call_code",
	"ListValidators":     "list_validators",
	"DumpConsensusState": "dump_consensus_state",
	"DumpStorage":        "dump_storage",
	"BroadcastTx":        "broadcast_tx",
	"ListUnconfirmedTxs": "list_unconfirmed_txs",
	"ListAccounts":       "list_accounts",
	"GetName":            "get_name",
	"ListNames":          "list_names",
	"GenPrivAccount":     "unsafe/gen_priv_account",
	"SignTx":             "unsafe/sign_tx",
}

/*
// fill the map from camelcase to lowercase
func fillReverseFuncMap() map[string]string {
	fMap := make(map[string]string)
	for name, f := range core.Routes {
		camelName := runtime.FuncForPC(f.f.Pointer()).Name()
		spl := strings.Split(camelName, ".")
		if len(spl) > 1 {
			camelName = spl[len(spl)-1]
		}
		fMap[camelName] = name
	}
	return fMap
}
*/

type Response struct {
	Status string
	Data   interface{}
	Error  string
}

//go:generate go-rpc-gen -interface Client -dir ../core -pkg core -type *ClientHTTP,*ClientJSON -exclude pipe.go -out-pkg core_client

type ClientJSON struct {
	addr string
}

type ClientHTTP struct {
	addr string
}

func NewClient(addr, typ string) Client {
	switch typ {
	case "HTTP":
		return &ClientHTTP{addr}
	case "JSONRPC":
		return &ClientJSON{addr}
	default:
		panic("Unknown client type " + typ + ". Select HTTP or JSONRPC")
	}
	return nil
}

func argsToJson(args ...interface{}) ([]string, error) {
	l := len(args)
	jsons := make([]string, l)
	n, err := new(int64), new(error)
	for i, a := range args {
		buf := new(bytes.Buffer)
		wire.WriteJSON(a, buf, n, err)
		if *err != nil {
			return nil, *err
		}
		jsons[i] = string(buf.Bytes())
	}
	return jsons, nil
}

func (c *ClientJSON) RequestResponse(s rpctypes.RPCRequest) (b []byte, err error) {
	b = wire.JSONBytes(s)
	buf := bytes.NewBuffer(b)
	resp, err := http.Post(c.addr, "text/json", buf)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

/*
	What follows is used by `rpc-gen` when `go generate` is called
	to populate the rpc client methods
*/

// first we define the base interface, which rpc-gen will further populate with generated methods

/*rpc-gen:define-interface Client
type Client interface {
	Address() string // returns the remote address
}
*/

// encoding functions

func binaryWriter(args ...interface{}) ([]interface{}, error) {
	list := []interface{}{}
	for _, a := range args {
		buf, n, err := new(bytes.Buffer), new(int64), new(error)
		wire.WriteJSON(a, buf, n, err)
		if *err != nil {
			return nil, *err
		}
		list = append(list, buf.Bytes())

	}
	return list, nil
}

func argsToURLValues(argNames []string, args ...interface{}) (url.Values, error) {
	values := make(url.Values)
	if len(argNames) == 0 {
		return values, nil
	}
	if len(argNames) != len(args) {
		return nil, fmt.Errorf("argNames and args have different lengths: %d, %d", len(argNames), len(args))
	}
	slice, err := argsToJson(args...)
	if err != nil {
		return nil, err
	}
	for i, name := range argNames {
		s := slice[i]
		values.Set(name, s) // s[0]
		/*for j := 1; j < len(s); j++ {
			values.Add(name, s[j])
		}*/
	}
	return values, nil
}

func unmarshalCheckResponse(body []byte) (response *ctypes.Response, err error) {
	response = new(ctypes.Response)
	wire.ReadJSON(response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}
	return response, nil
}

// import statements we will need for the templates

/*rpc-gen:imports:
rpctypes github.com/tendermint/tendermint/rpc/types
net/http
io/ioutil
fmt
*/

// Template functions to be filled in

/*rpc-gen:template:*ClientJSON func (c *ClientJSON) {{name}}({{args.def}}) ({{response}}) {
	request := rpctypes.RPCRequest{
		JSONRPC: "2.0",
		Method:  reverseFuncMap["{{name}}"],
		Params:  []interface{}{ {{args.ident}} },
		ID:      "",
	}
	body, err := c.RequestResponse(request)
	if err != nil{
		return nil, err
	}
	response, err := unmarshalCheckResponse(body)
	if err != nil{
		return nil, err
	}
	if response.Result == nil {
		return nil, nil
	}
	result, ok := response.Result.({{response.0}})
	if !ok{
		return nil, fmt.Errorf("response result was wrong type")
	}
	return result, nil
}*/

/*rpc-gen:template:*ClientHTTP func (c *ClientHTTP) {{name}}({{args.def}}) ({{response}}){
	values, err := argsToURLValues({{args.name}}, {{args.ident}})
	if err != nil{
		return nil, err
	}
	resp, err := http.PostForm(c.addr+reverseFuncMap["{{name}}"], values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	response, err := unmarshalCheckResponse(body)
	if err != nil{
		return nil, err
	}
	if response.Result == nil {
		return nil, nil
	}
	result, ok := response.Result.({{response.0}})
	if !ok{
		return nil, fmt.Errorf("response result was wrong type")
	}
	return result, nil
}*/
