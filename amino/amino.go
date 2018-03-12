package amino

import (
	amino "github.com/tendermint/go-amino"
)

/*
// Expose access to a global amino codec
// TODO: maybe introduce some Context object
// containing logger, config, codec that can
// be threaded through everything to avoid this global
var cdc *amino.Codec

func init() {
	cdc = amino.NewCodec()
	crypto.RegisterWire(cdc)
}
*/

// Just a flow through to go-amino.
// To be used later for the global codec

func MarshalBinary(o interface{}) ([]byte, error) {
	return amino.MarshalBinary(o)
}

func UnmarshalBinary(bz []byte, ptr interface{}) error {
	return amino.UnmarshalBinary(bz, ptr)
}

func MarshalJSON(o interface{}) ([]byte, error) {
	return amino.MarshalJSON(o)
}

func UnmarshalJSON(jsonBz []byte, ptr interface{}) error {
	return amino.UnmarshalJSON(jsonBz, ptr)
}

type ConcreteType = amino.ConcreteType

func RegisterInterface(o interface{}, ctypes ...ConcreteType) *amino.TypeInfo {
	return amino.RegisterInterface(o, ctypes...)
}

const RFC3339Millis = amino.RFC3339Millis

/*

func RegisterInterface(ptr interface{}, opts *amino.InterfaceOptions) {
	cdc.RegisterInterface(ptr, opts)
}

func RegisterConcrete(o interface{}, name string, opts *amino.ConcreteOptions) {
	cdc.RegisterConcrete(o, name, opts)
}

//-------------------------------

const RFC3339Millis = amino.RFC3339Millis
*/
