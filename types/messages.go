package types

import (
	"io"

	"github.com/golang/protobuf/proto"
	"github.com/tendermint/go-wire"
)

const (
	RequestTypeEcho      = uint32(0x01)
	RequestTypeFlush     = uint32(0x02)
	RequestTypeInfo      = uint32(0x03)
	RequestTypeSetOption = uint32(0x04)
	// reserved for GetOption = uint32(0x05)

	ResponseTypeException = uint32(0x10)
	ResponseTypeEcho      = uint32(0x11)
	ResponseTypeFlush     = uint32(0x12)
	ResponseTypeInfo      = uint32(0x13)
	ResponseTypeSetOption = uint32(0x14)
	// reserved for GetOption = uint32(0x15)

	RequestTypeAppendTx = uint32(0x21)
	RequestTypeCheckTx  = uint32(0x22)
	RequestTypeGetHash  = uint32(0x23)
	RequestTypeQuery    = uint32(0x24)

	ResponseTypeAppendTx = uint32(0x31)
	ResponseTypeCheckTx  = uint32(0x32)
	ResponseTypeGetHash  = uint32(0x33)
	ResponseTypeQuery    = uint32(0x34)
)

func RequestEcho(message string) *Request {
	return &Request{
		Type: RequestTypeEcho,
		Data: []byte(message),
	}
}

func RequestFlush() *Request {
	return &Request{
		Type: RequestTypeFlush,
	}
}

func RequestInfo() *Request {
	return &Request{
		Type: RequestTypeInfo,
	}
}

func RequestSetOption(key string, value string) *Request {
	return &Request{
		Type:  RequestTypeSetOption,
		Key:   key,
		Value: value,
	}
}

func RequestAppendTx(txBytes []byte) *Request {
	return &Request{
		Type: RequestTypeAppendTx,
		Data: txBytes,
	}
}

func RequestCheckTx(txBytes []byte) *Request {
	return &Request{
		Type: RequestTypeCheckTx,
		Data: txBytes,
	}
}

func RequestGetHash() *Request {
	return &Request{
		Type: RequestTypeGetHash,
	}
}

func RequestQuery(queryBytes []byte) *Request {
	return &Request{
		Type: RequestTypeQuery,
		Data: queryBytes,
	}
}

//----------------------------------------

func ResponseException(errStr string) *Response {
	return &Response{
		Type:  ResponseTypeException,
		Error: errStr,
	}
}

func ResponseEcho(message string) *Response {
	return &Response{
		Type: ResponseTypeEcho,
		Data: []byte(message),
	}
}

func ResponseFlush() *Response {
	return &Response{
		Type: ResponseTypeFlush,
	}
}

func ResponseInfo(info string) *Response {
	return &Response{
		Type: ResponseTypeInfo,
		Data: []byte(info),
	}
}

func ResponseSetOption(log string) *Response {
	return &Response{
		Type: ResponseTypeSetOption,
		Log:  log,
	}
}

func ResponseAppendTx(code RetCode, result []byte, log string) *Response {
	return &Response{
		Type: ResponseTypeAppendTx,
		Code: uint32(code),
		Data: result,
		Log:  log,
	}
}

func ResponseCheckTx(code RetCode, result []byte, log string) *Response {
	return &Response{
		Type: ResponseTypeCheckTx,
		Code: uint32(code),
		Data: result,
		Log:  log,
	}
}

func ResponseGetHash(hash []byte, log string) *Response {
	return &Response{
		Type: ResponseTypeGetHash,
		Data: hash,
		Log:  log,
	}
}

func ResponseQuery(code RetCode, result []byte, log string) *Response {
	return &Response{
		Type: ResponseTypeQuery,
		Code: uint32(code),
		Data: result,
		Log:  log,
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
