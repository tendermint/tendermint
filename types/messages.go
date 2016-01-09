package types

import "github.com/tendermint/go-wire"

const (
	requestTypeEcho      = byte(0x01)
	requestTypeFlush     = byte(0x02)
	requestTypeInfo      = byte(0x03)
	requestTypeSetOption = byte(0x04)
	// reserved for GetOption = byte(0x05)

	responseTypeException = byte(0x10)
	responseTypeEcho      = byte(0x11)
	responseTypeFlush     = byte(0x12)
	responseTypeInfo      = byte(0x13)
	responseTypeSetOption = byte(0x14)
	// reserved for GetOption = byte(0x15)

	requestTypeAppendTx    = byte(0x21)
	requestTypeCheckTx     = byte(0x22)
	requestTypeGetHash     = byte(0x23)
	requestTypeAddListener = byte(0x24)
	requestTypeRemListener = byte(0x25)
	// reserved for responseTypeEvent 0x26

	responseTypeAppendTx    = byte(0x31)
	responseTypeCheckTx     = byte(0x32)
	responseTypeGetHash     = byte(0x33)
	responseTypeAddListener = byte(0x34)
	responseTypeRemListener = byte(0x35)
	responseTypeEvent       = byte(0x36)
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

var _ = wire.RegisterInterface(
	struct{ Request }{},
	wire.ConcreteType{RequestEcho{}, requestTypeEcho},
	wire.ConcreteType{RequestFlush{}, requestTypeFlush},
	wire.ConcreteType{RequestInfo{}, requestTypeInfo},
	wire.ConcreteType{RequestSetOption{}, requestTypeSetOption},
	wire.ConcreteType{RequestAppendTx{}, requestTypeAppendTx},
	wire.ConcreteType{RequestCheckTx{}, requestTypeCheckTx},
	wire.ConcreteType{RequestGetHash{}, requestTypeGetHash},
	wire.ConcreteType{RequestAddListener{}, requestTypeAddListener},
	wire.ConcreteType{RequestRemListener{}, requestTypeRemListener},
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

var _ = wire.RegisterInterface(
	struct{ Response }{},
	wire.ConcreteType{ResponseEcho{}, responseTypeEcho},
	wire.ConcreteType{ResponseFlush{}, responseTypeFlush},
	wire.ConcreteType{ResponseInfo{}, responseTypeInfo},
	wire.ConcreteType{ResponseSetOption{}, responseTypeSetOption},
	wire.ConcreteType{ResponseAppendTx{}, responseTypeAppendTx},
	wire.ConcreteType{ResponseCheckTx{}, responseTypeCheckTx},
	wire.ConcreteType{ResponseGetHash{}, responseTypeGetHash},
	wire.ConcreteType{ResponseAddListener{}, responseTypeAddListener},
	wire.ConcreteType{ResponseRemListener{}, responseTypeRemListener},
	wire.ConcreteType{ResponseException{}, responseTypeException},
	wire.ConcreteType{ResponseEvent{}, responseTypeEvent},
)
