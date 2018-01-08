package data

import (
	"github.com/pkg/errors"
	wire "github.com/tendermint/go-wire"
)

type binaryMapper struct {
	base  interface{}
	impls []wire.ConcreteType
}

func newBinaryMapper(base interface{}) *binaryMapper {
	return &binaryMapper{
		base: base,
	}
}

// registerImplementation allows you to register multiple concrete types.
//
// We call wire.RegisterInterface with the entire (growing list) each time,
// as we do not know when the end is near.
func (m *binaryMapper) registerImplementation(data interface{}, kind string, b byte) {
	m.impls = append(m.impls, wire.ConcreteType{O: data, Byte: b})
	wire.RegisterInterface(m.base, m.impls...)
}

// ToWire is a convenience method to serialize with go-wire
// error is there to keep the same interface as json, but always nil
func ToWire(o interface{}) ([]byte, error) {
	return wire.BinaryBytes(o), nil
}

// FromWire is a convenience method to deserialize with go-wire
func FromWire(d []byte, o interface{}) error {
	return errors.WithStack(
		wire.ReadBinaryBytes(d, o))
}
