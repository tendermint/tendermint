package wire

import (
	crypto "github.com/tendermint/go-crypto"
	wire "github.com/tendermint/go-wire"
)

// Expose access to a global wire codec
// TODO: maybe introduce some Context object
// containing logger, config, codec that can
// be threaded through everything to avoid this global

var cdc *wire.Codec

func init() {
	cdc = wire.NewCodec()
	crypto.RegisterWire(cdc)
}

func MarshalBinary(o interface{}) ([]byte, error) {
	return cdc.MarshalBinary(o)
}

func UnmarshalBinary(bz []byte, ptr interface{}) error {
	return cdc.UnmarshalBinary(bz, ptr)
}

func MarshalJSON(o interface{}) ([]byte, error) {
	return cdc.MarshalJSON(o)
}

func UnmarshalJSON(jsonBz []byte, ptr interface{}) error {
	return cdc.UnmarshalJSON(jsonBz, ptr)
}

func RegisterInterface(ptr interface{}, opts *wire.InterfaceOptions) {
	cdc.RegisterInterface(ptr, opts)
}

func RegisterConcrete(o interface{}, name string, opts *wire.ConcreteOptions) {
	cdc.RegisterConcrete(o, name, opts)
}

//-------------------------------

const RFC3339Millis = wire.RFC3339Millis
