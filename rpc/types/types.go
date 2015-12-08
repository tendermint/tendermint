package rpctypes

type RPCRequest struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      string        `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

func NewRPCRequest(id string, method string, params []interface{}) RPCRequest {
	return RPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

type RPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Result  interface{} `json:"result"`
	Error   string      `json:"error"`
}

func NewRPCResponse(id string, res interface{}, err string) RPCResponse {
	return RPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  res,
		Error:   err,
	}
}
