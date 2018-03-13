package amino

import (
	"io"

	amino "github.com/tendermint/go-amino"
	crypto "github.com/tendermint/go-crypto"
)

// Expose access to a global amino codec
// TODO: maybe introduce some Context object
// containing logger, config, codec that can
// be threaded through everything to avoid this global
var cdc *amino.Codec

func init() {
	cdc = amino.NewCodec()
	crypto.RegisterAmino(cdc)
}

func MarshalBinary(o interface{}) ([]byte, error) {
	return cdc.MarshalBinary(o)
}

func MarshalBinaryBare(o interface{}) ([]byte, error) {
	return cdc.MarshalBinaryBare(o)
}

func UnmarshalBinary(bz []byte, ptr interface{}) error {
	return cdc.UnmarshalBinary(bz, ptr)
}

func UnmarshalBinaryReader(r io.Reader, ptr interface{}, maxSize int64) error {
	return cdc.UnmarshalBinaryReader(r, ptr, maxSize)
}

func UnmarshalBinaryBare(bz []byte, ptr interface{}) error {
	return cdc.UnmarshalBinaryBare(bz, ptr)
}

func MarshalJSON(o interface{}) ([]byte, error) {
	return cdc.MarshalJSON(o)
}

func UnmarshalJSON(jsonBz []byte, ptr interface{}) error {
	return cdc.UnmarshalJSON(jsonBz, ptr)
}

func RegisterInterface(ptr interface{}, opts *amino.InterfaceOptions) {
	cdc.RegisterInterface(ptr, opts)
}

func RegisterConcrete(o interface{}, name string, opts *amino.ConcreteOptions) {
	cdc.RegisterConcrete(o, name, opts)
}

//-------------------------------

const RFC3339Millis = amino.RFC3339Millis
