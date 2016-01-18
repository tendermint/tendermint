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

	RequestTypeAppendTx    = byte(0x21)
	RequestTypeCheckTx     = byte(0x22)
	RequestTypeGetHash     = byte(0x23)
	RequestTypeAddListener = byte(0x24)
	RequestTypeRemListener = byte(0x25)
	// reserved for ResponseTypeEvent 0x26
	RequestTypeQuery = byte(0x27)

	ResponseTypeAppendTx    = byte(0x31)
	ResponseTypeCheckTx     = byte(0x32)
	ResponseTypeGetHash     = byte(0x33)
	ResponseTypeAddListener = byte(0x34)
	ResponseTypeRemListener = byte(0x35)
	ResponseTypeEvent       = byte(0x36)
	ResponseTypeQuery       = byte(0x37)
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

type RequestAddListener struct {
	EventKey string
}

type RequestRemListener struct {
	EventKey string
}

type RequestQuery struct {
	QueryBytes []byte
}

type Request interface {
	AssertRequestType()
}

func (_ RequestEcho) AssertRequestType()        {}
func (_ RequestFlush) AssertRequestType()       {}
func (_ RequestInfo) AssertRequestType()        {}
func (_ RequestSetOption) AssertRequestType()   {}
func (_ RequestAppendTx) AssertRequestType()    {}
func (_ RequestCheckTx) AssertRequestType()     {}
func (_ RequestGetHash) AssertRequestType()     {}
func (_ RequestAddListener) AssertRequestType() {}
func (_ RequestRemListener) AssertRequestType() {}
func (_ RequestQuery) AssertRequestType()       {}

var _ = wire.RegisterInterface(
	struct{ Request }{},
	wire.ConcreteType{RequestEcho{}, RequestTypeEcho},
	wire.ConcreteType{RequestFlush{}, RequestTypeFlush},
	wire.ConcreteType{RequestInfo{}, RequestTypeInfo},
	wire.ConcreteType{RequestSetOption{}, RequestTypeSetOption},
	wire.ConcreteType{RequestAppendTx{}, RequestTypeAppendTx},
	wire.ConcreteType{RequestCheckTx{}, RequestTypeCheckTx},
	wire.ConcreteType{RequestGetHash{}, RequestTypeGetHash},
	wire.ConcreteType{RequestAddListener{}, RequestTypeAddListener},
	wire.ConcreteType{RequestRemListener{}, RequestTypeRemListener},
	wire.ConcreteType{RequestQuery{}, RequestTypeQuery},
)

//----------------------------------------

type ResponseEcho struct {
	Message string
}

type ResponseFlush struct {
}

type ResponseInfo struct {
	Data []string
}

type ResponseSetOption struct {
	RetCode
}

type ResponseAppendTx struct {
	RetCode
}

type ResponseCheckTx struct {
	RetCode
}

type ResponseGetHash struct {
	RetCode
	Hash []byte
}

type ResponseAddListener struct {
	RetCode
}

type ResponseRemListener struct {
	RetCode
}

type ResponseException struct {
	Error string
}

type ResponseEvent struct {
	Event
}

type ResponseQuery struct {
	RetCode
	Result []byte
}

type Response interface {
	AssertResponseType()
}

func (_ ResponseEcho) AssertResponseType()        {}
func (_ ResponseFlush) AssertResponseType()       {}
func (_ ResponseInfo) AssertResponseType()        {}
func (_ ResponseSetOption) AssertResponseType()   {}
func (_ ResponseAppendTx) AssertResponseType()    {}
func (_ ResponseCheckTx) AssertResponseType()     {}
func (_ ResponseGetHash) AssertResponseType()     {}
func (_ ResponseAddListener) AssertResponseType() {}
func (_ ResponseRemListener) AssertResponseType() {}
func (_ ResponseException) AssertResponseType()   {}
func (_ ResponseEvent) AssertResponseType()       {}
func (_ ResponseQuery) AssertResponseType()       {}

var _ = wire.RegisterInterface(
	struct{ Response }{},
	wire.ConcreteType{ResponseEcho{}, ResponseTypeEcho},
	wire.ConcreteType{ResponseFlush{}, ResponseTypeFlush},
	wire.ConcreteType{ResponseInfo{}, ResponseTypeInfo},
	wire.ConcreteType{ResponseSetOption{}, ResponseTypeSetOption},
	wire.ConcreteType{ResponseAppendTx{}, ResponseTypeAppendTx},
	wire.ConcreteType{ResponseCheckTx{}, ResponseTypeCheckTx},
	wire.ConcreteType{ResponseGetHash{}, ResponseTypeGetHash},
	wire.ConcreteType{ResponseAddListener{}, ResponseTypeAddListener},
	wire.ConcreteType{ResponseRemListener{}, ResponseTypeRemListener},
	wire.ConcreteType{ResponseException{}, ResponseTypeException},
	wire.ConcreteType{ResponseEvent{}, ResponseTypeEvent},
	wire.ConcreteType{ResponseQuery{}, ResponseTypeQuery},
)
