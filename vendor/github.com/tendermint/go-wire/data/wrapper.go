package data

import "fmt"

/*
Mapper is the main entry point in the package.

On init, you should call NewMapper() for each interface type you want
to support flexible de-serialization, and then
RegisterImplementation() in the init() function for each implementation of these
interfaces.

Note that unlike go-wire, you can call RegisterImplementation separately from
different locations with each implementation, not all in one place.
Just be careful not to use the same key or byte, of init will *panic*
*/
type Mapper struct {
	*jsonMapper
	*binaryMapper
}

// NewMapper creates a Mapper.
//
// If you have:
//   type Foo interface {....}
//   type FooS struct { Foo }
// then you should pass in FooS{} in NewMapper, and implementations of Foo
// in RegisterImplementation
func NewMapper(base interface{}) Mapper {
	return Mapper{
		jsonMapper:   newJsonMapper(base),
		binaryMapper: newBinaryMapper(base),
	}
}

// RegisterImplementation should be called once for each implementation of the
// interface that we wish to support.
//
// kind is the type string used in the json representation, while b is the
// type byte used in the go-wire representation. data is one instance of this
// concrete type, like Bar{}
func (m Mapper) RegisterImplementation(data interface{}, kind string, b byte) Mapper {
	m.jsonMapper.registerImplementation(data, kind, b)
	m.binaryMapper.registerImplementation(data, kind, b)
	return m
}

// ToText is a rather special-case serialization for cli, especially for []byte
// interfaces
//
// It tries to serialize as json, and the result looks like:
//   { "type": "string", "data": "string" }
// Then it will return "<type>:<data>"
//
// Main usecase is serializing eg. crypto.PubKeyS as "ed25119:a1b2c3d4..."
// for displaying in cli tools.
//
// It also supports encoding data.Bytes to a string using the proper codec
// (or anything else that has a marshals to a string)
func ToText(o interface{}) (string, error) {
	d, err := ToJSON(o)
	if err != nil {
		return "", err
	}

	// try to recover as a string (data.Bytes case)
	var s string
	err = FromJSON(d, &s)
	if err == nil {
		return s, nil
	}

	// if not, then try to recover as an interface (crypto.*S case)
	text := textEnv{}
	err = FromJSON(d, &text)
	if err != nil {
		return "", err
	}
	res := fmt.Sprintf("%s:%s", text.Kind, text.Data)
	return res, nil
}

// textEnv lets us parse an envelope if the data was output as string
// (either text, or a serialized []byte)
type textEnv struct {
	Kind string `json:"type"`
	Data string `json:"data"`
}
