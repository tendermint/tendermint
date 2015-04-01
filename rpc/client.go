package rpc

import (
	"bytes"
	"fmt"
	"github.com/tendermint/tendermint2/binary"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	// Uncomment to use go:generate
	// _ "github.com/ebuchman/go-rpc-gen"
)

type Response struct {
	Status string
	Data   interface{}
	Error  string
}

//go:generate go-rpc-gen -interface Client -pkg core -type *ClientHTTP,*ClientJSON -exclude pipe.go -out-pkg rpc

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
	}
	return nil
}

func argsToJson(args ...interface{}) ([][]string, error) {
	l := len(args)
	jsons := make([][]string, l)
	n, err := new(int64), new(error)
	for i, a := range args {
		//if its a slice, we serliaze separately and pack into a slice of strings
		// otherwise its a slice of length 1
		if v := reflect.ValueOf(a); v.Kind() == reflect.Slice {
			ty := v.Type()
			rt := ty.Elem()
			if rt.Kind() == reflect.Uint8 {
				buf := new(bytes.Buffer)
				binary.WriteJSON(a, buf, n, err)
				if *err != nil {
					return nil, *err
				}
				jsons[i] = []string{string(buf.Bytes())}
			} else {
				slice := make([]string, v.Len())
				for j := 0; j < v.Len(); j++ {
					buf := new(bytes.Buffer)
					binary.WriteJSON(v.Index(j).Interface(), buf, n, err)
					if *err != nil {
						return nil, *err
					}
					slice[j] = string(buf.Bytes())
				}
				jsons[i] = slice
			}
		} else {
			buf := new(bytes.Buffer)
			binary.WriteJSON(a, buf, n, err)
			if *err != nil {
				return nil, *err
			}
			jsons[i] = []string{string(buf.Bytes())}
		}
	}
	return jsons, nil
}

func (c *ClientHTTP) RequestResponse(method string, values url.Values) (*Response, error) {
	resp, err := http.PostForm(c.addr+method, values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	response := new(Response)
	fmt.Println(string(body))
	binary.ReadJSON(response, body, &err)
	if err != nil {
		return nil, err
	}
	fmt.Println(response.Data)
	return response, nil
}

func (c *ClientJSON) RequestResponse(s RPCRequest) (b []byte, err error) {
	b = binary.JSONBytes(s)
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
		binary.WriteJSON(a, buf, n, err)
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
		values.Set(name, s[0])
		for i := 1; i < len(s); i++ {
			values.Add(name, s[i])
		}
	}
	return values, nil
}

// import statements we will need for the templates

/*rpc-gen:imports:
github.com/tendermint/tendermint2/binary
net/http
io/ioutil
fmt
*/

// Template functions to be filled in

/*rpc-gen:template:*ClientJSON func (c *ClientJSON) {{name}}({{args.def}}) ({{response}}) {
	request := RPCRequest{
		JSONRPC: "2.0",
		Method:  {{lowername}},
		Params:  []interface{}{ {{args.ident}} },
		Id:      0,
	}
	body, err := c.RequestResponse(request)
	if err != nil{
		return nil, err
	}
	var response struct {
		Result {{response.0}} `json:"result"`
		Error  string `json:"error"`
		Id string `json:"id"`
		JSONRPC string `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != ""{
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}*/

/*rpc-gen:template:*ClientHTTP func (c *ClientHTTP) {{name}}({{args.def}}) ({{response}}){
	values, err := argsToURLValues({{args.name}}, {{args.ident}})
	if err != nil{
		return nil, err
	}
	resp, err := http.PostForm(c.addr+{{lowername}}, values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var response struct {
		Result {{response.0}} `json:"result"`
		Error  string `json:"error"`
		Id string `json:"id"`
		JSONRPC string `json:"jsonrpc"`
	}
	binary.ReadJSON(&response, body, &err)
	if err != nil {
		return nil, err
	}
	if response.Error != ""{
		return nil, fmt.Errorf(response.Error)
	}
	return response.Result, nil
}*/
