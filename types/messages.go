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
	requestTypeGetHash     = byte(0x22)
	requestTypeCommit      = byte(0x23)
	requestTypeRollback    = byte(0x24)
	requestTypeAddListener = byte(0x25)
	requestTypeRemListener = byte(0x26)
	// reserved for responseTypeEvent 0x27

	responseTypeAppendTx    = byte(0x31)
	responseTypeGetHash     = byte(0x32)
	responseTypeCommit      = byte(0x33)
	responseTypeRollback    = byte(0x34)
	responseTypeAddListener = byte(0x35)
	responseTypeRemListener = byte(0x36)
	responseTypeEvent       = byte(0x37)
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

type RequestGetHash struct {
}

type RequestCommit struct {
}

type RequestRollback struct {
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
func (_ RequestGetHash) AssertRequestType()     {}
func (_ RequestCommit) AssertRequestType()      {}
func (_ RequestRollback) AssertRequestType()    {}
func (_ RequestAddListener) AssertRequestType() {}
func (_ RequestRemListener) AssertRequestType() {}

var _ = wire.RegisterInterface(
	struct{ Request }{},
	wire.ConcreteType{RequestEcho{}, requestTypeEcho},
	wire.ConcreteType{RequestFlush{}, requestTypeFlush},
	wire.ConcreteType{RequestInfo{}, requestTypeInfo},
	wire.ConcreteType{RequestSetOption{}, requestTypeSetOption},
	wire.ConcreteType{RequestAppendTx{}, requestTypeAppendTx},
	wire.ConcreteType{RequestGetHash{}, requestTypeGetHash},
	wire.ConcreteType{RequestCommit{}, requestTypeCommit},
	wire.ConcreteType{RequestRollback{}, requestTypeRollback},
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

type ResponseGetHash struct {
	RetCode
	Hash []byte
}

type ResponseCommit struct {
	RetCode
}

type ResponseRollback struct {
	RetCode
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
func (_ ResponseGetHash) AssertResponseType()     {}
func (_ ResponseCommit) AssertResponseType()      {}
func (_ ResponseRollback) AssertResponseType()    {}
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
	wire.ConcreteType{ResponseGetHash{}, responseTypeGetHash},
	wire.ConcreteType{ResponseCommit{}, responseTypeCommit},
	wire.ConcreteType{ResponseRollback{}, responseTypeRollback},
	wire.ConcreteType{ResponseAddListener{}, responseTypeAddListener},
	wire.ConcreteType{ResponseRemListener{}, responseTypeRemListener},
	wire.ConcreteType{ResponseException{}, responseTypeException},
	wire.ConcreteType{ResponseEvent{}, responseTypeEvent},
)
