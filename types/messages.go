package types

import (
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/tendermint/go-wire"
)

func ToRequestEcho(message string) *Request {
	return &Request{
		Value: &Request_Echo{&RequestEcho{message}},
	}
}

func ToRequestFlush() *Request {
	return &Request{
		Value: &Request_Flush{&RequestFlush{}},
	}
}

func ToRequestInfo() *Request {
	return &Request{
		Value: &Request_Info{&RequestInfo{}},
	}
}

func ToRequestSetOption(key string, value string) *Request {
	return &Request{
		Value: &Request_SetOption{&RequestSetOption{key, value}},
	}
}

func ToRequestDeliverTx(txBytes []byte) *Request {
	return &Request{
		Value: &Request_DeliverTx{&RequestDeliverTx{txBytes}},
	}
}

func ToRequestCheckTx(txBytes []byte) *Request {
	return &Request{
		Value: &Request_CheckTx{&RequestCheckTx{txBytes}},
	}
}

func ToRequestCommit() *Request {
	return &Request{
		Value: &Request_Commit{&RequestCommit{}},
	}
}

func ToRequestQuery(reqQuery RequestQuery) *Request {
	return &Request{
		Value: &Request_Query{&reqQuery},
	}
}

func ToRequestInitChain(validators []*Validator) *Request {
	return &Request{
		Value: &Request_InitChain{&RequestInitChain{validators}},
	}
}

func ToRequestBeginBlock(hash []byte, header *Header) *Request {
	return &Request{
		Value: &Request_BeginBlock{&RequestBeginBlock{hash, header}},
	}
}

func ToRequestEndBlock(height uint64) *Request {
	return &Request{
		Value: &Request_EndBlock{&RequestEndBlock{height}},
	}
}

//----------------------------------------

func ToResponseException(errStr string) *Response {
	return &Response{
		Value: &Response_Exception{&ResponseException{errStr}},
	}
}

func ToResponseEcho(message string) *Response {
	return &Response{
		Value: &Response_Echo{&ResponseEcho{message}},
	}
}

func ToResponseFlush() *Response {
	return &Response{
		Value: &Response_Flush{&ResponseFlush{}},
	}
}

func ToResponseInfo(resInfo ResponseInfo) *Response {
	return &Response{
		Value: &Response_Info{&resInfo},
	}
}

func ToResponseSetOption(log string) *Response {
	return &Response{
		Value: &Response_SetOption{&ResponseSetOption{log}},
	}
}

func ToResponseDeliverTx(code CodeType, data []byte, log string) *Response {
	return &Response{
		Value: &Response_DeliverTx{&ResponseDeliverTx{code, data, log}},
	}
}

func ToResponseCheckTx(code CodeType, data []byte, log string) *Response {
	return &Response{
		Value: &Response_CheckTx{&ResponseCheckTx{code, data, log}},
	}
}

func ToResponseCommit(code CodeType, data []byte, log string) *Response {
	return &Response{
		Value: &Response_Commit{&ResponseCommit{code, data, log}},
	}
}

func ToResponseQuery(resQuery ResponseQuery) *Response {
	return &Response{
		Value: &Response_Query{&resQuery},
	}
}

func ToResponseInitChain() *Response {
	return &Response{
		Value: &Response_InitChain{&ResponseInitChain{}},
	}
}

func ToResponseBeginBlock() *Response {
	return &Response{
		Value: &Response_BeginBlock{&ResponseBeginBlock{}},
	}
}

func ToResponseEndBlock(resEndBlock ResponseEndBlock) *Response {
	return &Response{
		Value: &Response_EndBlock{&resEndBlock},
	}
}

//----------------------------------------

// Write proto message, length delimited
func WriteMessage(msg proto.Message, w io.Writer) error {
	bz, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	var n int
	wire.WriteByteSlice(bz, w, &n, &err)
	return err
}

// Read proto message, length delimited
func ReadMessage(r io.Reader, msg proto.Message) error {
	var n int
	var err error
	bz := wire.ReadByteSlice(r, 0, &n, &err)
	if err != nil {
		return err
	}
	err = proto.Unmarshal(bz, msg)
	return err
}
