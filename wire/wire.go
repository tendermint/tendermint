package wire

import (
	"github.com/tendermint/go-wire"
)

/*
// Expose access to a global wire codec
// TODO: maybe introduce some Context object
// containing logger, config, codec that can
// be threaded through everything to avoid this global
var cdc *wire.Codec

func init() {
	cdc = wire.NewCodec()
	crypto.RegisterWire(cdc)
}
*/

// Just a flow through to go-wire.
// To be used later for the global codec

func MarshalBinary(o interface{}) ([]byte, error) {
	return wire.MarshalBinary(o)
}

func UnmarshalBinary(bz []byte, ptr interface{}) error {
	return wire.UnmarshalBinary(bz, ptr)
}

func MarshalJSON(o interface{}) ([]byte, error) {
	return wire.MarshalJSON(o)
}

func UnmarshalJSON(jsonBz []byte, ptr interface{}) error {
	return wire.UnmarshalJSON(jsonBz, ptr)
}

type ConcreteType = wire.ConcreteType

func RegisterInterface(o interface{}, ctypes ...ConcreteType) *wire.TypeInfo {
	return wire.RegisterInterface(o, ctypes...)
}

const RFC3339Millis = wire.RFC3339Millis

/*

func RegisterInterface(ptr interface{}, opts *wire.InterfaceOptions) {
	cdc.RegisterInterface(ptr, opts)
}

func RegisterConcrete(o interface{}, name string, opts *wire.ConcreteOptions) {
	cdc.RegisterConcrete(o, name, opts)
}

//-------------------------------

const RFC3339Millis = wire.RFC3339Millis
*/
