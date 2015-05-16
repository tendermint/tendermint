package rpctypes

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

// for requests coming in
type WSRequest struct {
	Type  string `json:"type"` // subscribe or unsubscribe
	Event string `json:"event"`
}

// for responses going out
type WSResponse struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
	Error string      `json:"error"`
}
