package types

import (
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/tendermint/go-wire"
)

func RequestEcho(message string) *Request {
	return &Request{
		Type: MessageType_Echo,
		Data: []byte(message),
	}
}

func RequestFlush() *Request {
	return &Request{
		Type: MessageType_Flush,
	}
}

func RequestInfo() *Request {
	return &Request{
		Type: MessageType_Info,
	}
}

func RequestSetOption(key string, value string) *Request {
	return &Request{
		Type:  MessageType_SetOption,
		Key:   key,
		Value: value,
	}
}

func RequestAppendTx(txBytes []byte) *Request {
	return &Request{
		Type: MessageType_AppendTx,
		Data: txBytes,
	}
}

func RequestCheckTx(txBytes []byte) *Request {
	return &Request{
		Type: MessageType_CheckTx,
		Data: txBytes,
	}
}

func RequestCommit() *Request {
	return &Request{
		Type: MessageType_Commit,
	}
}

func RequestQuery(queryBytes []byte) *Request {
	return &Request{
		Type: MessageType_Query,
		Data: queryBytes,
	}
}

func RequestInitChain(validators []*Validator) *Request {
	return &Request{
		Type:       MessageType_InitChain,
		Validators: validators,
	}
}

func RequestBeginBlock(height uint64) *Request {
	return &Request{
		Type:   MessageType_BeginBlock,
		Height: height,
	}
}

func RequestEndBlock(height uint64) *Request {
	return &Request{
		Type:   MessageType_EndBlock,
		Height: height,
	}
}

//----------------------------------------

func ResponseException(errStr string) *Response {
	return &Response{
		Type:  MessageType_Exception,
		Error: errStr,
	}
}

func ResponseEcho(message string) *Response {
	return &Response{
		Type: MessageType_Echo,
		Data: []byte(message),
	}
}

func ResponseFlush() *Response {
	return &Response{
		Type: MessageType_Flush,
	}
}

func ResponseInfo(info string) *Response {
	return &Response{
		Type: MessageType_Info,
		Data: []byte(info),
	}
}

func ResponseSetOption(log string) *Response {
	return &Response{
		Type: MessageType_SetOption,
		Log:  log,
	}
}

func ResponseAppendTx(code CodeType, data []byte, log string) *Response {
	return &Response{
		Type: MessageType_AppendTx,
		Code: code,
		Data: data,
		Log:  log,
	}
}

func ResponseCheckTx(code CodeType, data []byte, log string) *Response {
	return &Response{
		Type: MessageType_CheckTx,
		Code: code,
		Data: data,
		Log:  log,
	}
}

func ResponseCommit(code CodeType, data []byte, log string) *Response {
	return &Response{
		Type: MessageType_Commit,
		Code: code,
		Data: data,
		Log:  log,
	}
}

func ResponseQuery(code CodeType, data []byte, log string) *Response {
	return &Response{
		Type: MessageType_Query,
		Code: code,
		Data: data,
		Log:  log,
	}
}

func ResponseInitChain() *Response {
	return &Response{
		Type: MessageType_InitChain,
	}
}

func ResponseBeginBlock() *Response {
	return &Response{
		Type: MessageType_BeginBlock,
	}
}

func ResponseEndBlock(validators []*Validator) *Response {
	return &Response{
		Type:       MessageType_EndBlock,
		Validators: validators,
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
