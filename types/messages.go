package types

import "github.com/tendermint/go-wire"

const (
	requestTypeEcho          = byte(0x01)
	requestTypeFlush         = byte(0x02)
	requestTypeAppendTx      = byte(0x03)
	requestTypeGetHash       = byte(0x04)
	requestTypeCommit        = byte(0x05)
	requestTypeRollback      = byte(0x06)
	requestTypeSetEventsMode = byte(0x07)
	requestTypeAddListener   = byte(0x08)
	requestTypeRemListener   = byte(0x09)

	responseTypeEcho          = byte(0x11)
	responseTypeFlush         = byte(0x12)
	responseTypeAppendTx      = byte(0x13)
	responseTypeGetHash       = byte(0x14)
	responseTypeCommit        = byte(0x15)
	responseTypeRollback      = byte(0x16)
	responseTypeSetEventsMode = byte(0x17)
	responseTypeAddListener   = byte(0x18)
	responseTypeRemListener   = byte(0x19)

	responseTypeException = byte(0x20)
	responseTypeEvent     = byte(0x21)
)

//----------------------------------------

type RequestEcho struct {
	Message string
}

type RequestFlush struct {
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

type RequestSetEventsMode struct {
	EventsMode
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

func (_ RequestEcho) AssertRequestType()          {}
func (_ RequestFlush) AssertRequestType()         {}
func (_ RequestAppendTx) AssertRequestType()      {}
func (_ RequestGetHash) AssertRequestType()       {}
func (_ RequestCommit) AssertRequestType()        {}
func (_ RequestRollback) AssertRequestType()      {}
func (_ RequestSetEventsMode) AssertRequestType() {}
func (_ RequestAddListener) AssertRequestType()   {}
func (_ RequestRemListener) AssertRequestType()   {}

var _ = wire.RegisterInterface(
	struct{ Request }{},
	wire.ConcreteType{RequestEcho{}, requestTypeEcho},
	wire.ConcreteType{RequestFlush{}, requestTypeFlush},
	wire.ConcreteType{RequestAppendTx{}, requestTypeAppendTx},
	wire.ConcreteType{RequestGetHash{}, requestTypeGetHash},
	wire.ConcreteType{RequestCommit{}, requestTypeCommit},
	wire.ConcreteType{RequestRollback{}, requestTypeRollback},
	wire.ConcreteType{RequestSetEventsMode{}, requestTypeSetEventsMode},
	wire.ConcreteType{RequestAddListener{}, requestTypeAddListener},
	wire.ConcreteType{RequestRemListener{}, requestTypeRemListener},
)

//----------------------------------------

type ResponseEcho struct {
	RetCode
	Message string
}

type ResponseFlush struct {
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

type ResponseSetEventsMode struct {
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

func (_ ResponseEcho) AssertResponseType()          {}
func (_ ResponseFlush) AssertResponseType()         {}
func (_ ResponseAppendTx) AssertResponseType()      {}
func (_ ResponseGetHash) AssertResponseType()       {}
func (_ ResponseCommit) AssertResponseType()        {}
func (_ ResponseRollback) AssertResponseType()      {}
func (_ ResponseSetEventsMode) AssertResponseType() {}
func (_ ResponseAddListener) AssertResponseType()   {}
func (_ ResponseRemListener) AssertResponseType()   {}
func (_ ResponseException) AssertResponseType()     {}
func (_ ResponseEvent) AssertResponseType()         {}

var _ = wire.RegisterInterface(
	struct{ Response }{},
	wire.ConcreteType{ResponseEcho{}, responseTypeEcho},
	wire.ConcreteType{ResponseFlush{}, responseTypeFlush},
	wire.ConcreteType{ResponseAppendTx{}, responseTypeAppendTx},
	wire.ConcreteType{ResponseGetHash{}, responseTypeGetHash},
	wire.ConcreteType{ResponseCommit{}, responseTypeCommit},
	wire.ConcreteType{ResponseRollback{}, responseTypeRollback},
	wire.ConcreteType{ResponseSetEventsMode{}, responseTypeSetEventsMode},
	wire.ConcreteType{ResponseAddListener{}, responseTypeAddListener},
	wire.ConcreteType{ResponseRemListener{}, responseTypeRemListener},
	wire.ConcreteType{ResponseException{}, responseTypeException},
	wire.ConcreteType{ResponseEvent{}, responseTypeEvent},
)
