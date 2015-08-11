package rpctypes

type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	Id      string        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type RPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	Id      string      `json:"id"`
	Result  interface{} `json:"result"`
	Error   string      `json:"error"`
}

func NewRPCResponse(id string, res interface{}, err string) RPCResponse {
	return RPCResponse{
		JSONRPC: "2.0",
		Id:      id,
		Result:  res,
		Error:   err,
	}
}
