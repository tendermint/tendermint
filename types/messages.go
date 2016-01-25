package types

import "github.com/tendermint/go-wire"

const (
	RequestTypeEcho      = byte(0x01)
	RequestTypeFlush     = byte(0x02)
	RequestTypeInfo      = byte(0x03)
	RequestTypeSetOption = byte(0x04)
	// reserved for GetOption = byte(0x05)

	ResponseTypeException = byte(0x10)
	ResponseTypeEcho      = byte(0x11)
	ResponseTypeFlush     = byte(0x12)
	ResponseTypeInfo      = byte(0x13)
	ResponseTypeSetOption = byte(0x14)
	// reserved for GetOption = byte(0x15)

	RequestTypeAppendTx = byte(0x21)
	RequestTypeCheckTx  = byte(0x22)
	RequestTypeGetHash  = byte(0x23)
	RequestTypeQuery    = byte(0x24)

	ResponseTypeAppendTx = byte(0x31)
	ResponseTypeCheckTx  = byte(0x32)
	ResponseTypeGetHash  = byte(0x33)
	ResponseTypeQuery    = byte(0x34)
)

//----------------------------------------

type RequestEcho struct {
	Message string
}

type RequestFlush struct {
}

type RequestInfo struct {
}

type RequestSetOption struct {
	Key   string
	Value string
}

type RequestAppendTx struct {
	TxBytes []byte
}

type RequestCheckTx struct {
	TxBytes []byte
}

type RequestGetHash struct {
}

type RequestQuery struct {
	QueryBytes []byte
}

type Request interface {
	AssertRequestType()
}

func (_ RequestEcho) AssertRequestType()      {}
func (_ RequestFlush) AssertRequestType()     {}
func (_ RequestInfo) AssertRequestType()      {}
func (_ RequestSetOption) AssertRequestType() {}
func (_ RequestAppendTx) AssertRequestType()  {}
func (_ RequestCheckTx) AssertRequestType()   {}
func (_ RequestGetHash) AssertRequestType()   {}
func (_ RequestQuery) AssertRequestType()     {}

var _ = wire.RegisterInterface(
	struct{ Request }{},
	wire.ConcreteType{RequestEcho{}, RequestTypeEcho},
	wire.ConcreteType{RequestFlush{}, RequestTypeFlush},
	wire.ConcreteType{RequestInfo{}, RequestTypeInfo},
	wire.ConcreteType{RequestSetOption{}, RequestTypeSetOption},
	wire.ConcreteType{RequestAppendTx{}, RequestTypeAppendTx},
	wire.ConcreteType{RequestCheckTx{}, RequestTypeCheckTx},
	wire.ConcreteType{RequestGetHash{}, RequestTypeGetHash},
	wire.ConcreteType{RequestQuery{}, RequestTypeQuery},
)

//----------------------------------------

type ResponseException struct {
	Error string
}

type ResponseEcho struct {
	Message string
}

type ResponseFlush struct {
}

type ResponseInfo struct {
	Info string
}

type ResponseSetOption struct {
	Log string
}

type ResponseAppendTx struct {
	Code   RetCode
	Result []byte
	Log    string
}

type ResponseCheckTx struct {
	Code   RetCode
	Result []byte
	Log    string
}

type ResponseGetHash struct {
	Hash []byte
	Log  string
}

type ResponseQuery struct {
	Result []byte
	Log    string
}

type Response interface {
	AssertResponseType()
}

func (_ ResponseEcho) AssertResponseType()      {}
func (_ ResponseFlush) AssertResponseType()     {}
func (_ ResponseInfo) AssertResponseType()      {}
func (_ ResponseSetOption) AssertResponseType() {}
func (_ ResponseAppendTx) AssertResponseType()  {}
func (_ ResponseCheckTx) AssertResponseType()   {}
func (_ ResponseGetHash) AssertResponseType()   {}
func (_ ResponseException) AssertResponseType() {}
func (_ ResponseQuery) AssertResponseType()     {}

var _ = wire.RegisterInterface(
	struct{ Response }{},
	wire.ConcreteType{ResponseEcho{}, ResponseTypeEcho},
	wire.ConcreteType{ResponseFlush{}, ResponseTypeFlush},
	wire.ConcreteType{ResponseInfo{}, ResponseTypeInfo},
	wire.ConcreteType{ResponseSetOption{}, ResponseTypeSetOption},
	wire.ConcreteType{ResponseAppendTx{}, ResponseTypeAppendTx},
	wire.ConcreteType{ResponseCheckTx{}, ResponseTypeCheckTx},
	wire.ConcreteType{ResponseGetHash{}, ResponseTypeGetHash},
	wire.ConcreteType{ResponseException{}, ResponseTypeException},
	wire.ConcreteType{ResponseQuery{}, ResponseTypeQuery},
)
