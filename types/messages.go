package types

import (
	"io"

	"github.com/gogo/protobuf/proto"
	wire "github.com/tendermint/go-wire"
)

// WriteMessage writes a length-delimited protobuf message.
func WriteMessage(msg proto.Message, w io.Writer) error {
	bz, err := proto.Marshal(msg)
	if err != nil {
		return err
	}
	var n int
	wire.WriteByteSlice(bz, w, &n, &err)
	return err
}

// ReadMessage reads a length delimited protobuf message.
func ReadMessage(r io.Reader, msg proto.Message) error {
	var n int
	var err error
	bz := wire.ReadByteSlice(r, 0, &n, &err) //XXX: no max
	if err != nil {
		return err
	}
	err = proto.Unmarshal(bz, msg)
	return err
}

//----------------------------------------

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

func ToRequestInfo(req RequestInfo) *Request {
	return &Request{
		Value: &Request_Info{&req},
	}
}

func ToRequestSetOption(req RequestSetOption) *Request {
	return &Request{
		Value: &Request_SetOption{&req},
	}
}

func ToRequestDeliverTx(tx []byte) *Request {
	return &Request{
		Value: &Request_DeliverTx{&RequestDeliverTx{tx}},
	}
}

func ToRequestCheckTx(tx []byte) *Request {
	return &Request{
		Value: &Request_CheckTx{&RequestCheckTx{tx}},
	}
}

func ToRequestCommit() *Request {
	return &Request{
		Value: &Request_Commit{&RequestCommit{}},
	}
}

func ToRequestQuery(req RequestQuery) *Request {
	return &Request{
		Value: &Request_Query{&req},
	}
}

func ToRequestInitChain(req RequestInitChain) *Request {
	return &Request{
		Value: &Request_InitChain{&req},
	}
}

func ToRequestBeginBlock(req RequestBeginBlock) *Request {
	return &Request{
		Value: &Request_BeginBlock{&req},
	}
}

func ToRequestEndBlock(req RequestEndBlock) *Request {
	return &Request{
		Value: &Request_EndBlock{&req},
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

func ToResponseInfo(res ResponseInfo) *Response {
	return &Response{
		Value: &Response_Info{&res},
	}
}

func ToResponseSetOption(res ResponseSetOption) *Response {
	return &Response{
		Value: &Response_SetOption{&res},
	}
}

func ToResponseDeliverTx(res ResponseDeliverTx) *Response {
	return &Response{
		Value: &Response_DeliverTx{&res},
	}
}

func ToResponseCheckTx(res ResponseCheckTx) *Response {
	return &Response{
		Value: &Response_CheckTx{&res},
	}
}

func ToResponseCommit(res ResponseCommit) *Response {
	return &Response{
		Value: &Response_Commit{&res},
	}
}

func ToResponseQuery(res ResponseQuery) *Response {
	return &Response{
		Value: &Response_Query{&res},
	}
}

func ToResponseInitChain(res ResponseInitChain) *Response {
	return &Response{
		Value: &Response_InitChain{&res},
	}
}

func ToResponseBeginBlock(res ResponseBeginBlock) *Response {
	return &Response{
		Value: &Response_BeginBlock{&res},
	}
}

func ToResponseEndBlock(res ResponseEndBlock) *Response {
	return &Response{
		Value: &Response_EndBlock{&res},
	}
}
