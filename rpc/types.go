package rpc

type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	Id      int           `json:"id"`
}

type RPCResponse struct {
	Result  interface{} `json:"result"`
	Error   string      `json:"error"`
	Id      string      `json:"id"`
	JSONRPC string      `json:"jsonrpc"`
}

func NewRPCResponse(res interface{}, err string) RPCResponse {
	if res == nil {
		res = struct{}{}
	}
	return RPCResponse{
		Result:  res,
		Error:   err,
		Id:      "",
		JSONRPC: "2.0",
	}
}
