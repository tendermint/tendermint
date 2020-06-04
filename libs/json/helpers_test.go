package json_test

import (
	"time"

	"github.com/tendermint/tendermint/libs/json"
)

// Register Testa, an instance of the Car interface.
func init() {
	json.RegisterType(Tesla{}, "car/tesla")
}

type Car interface {
	Drive() error
}

type Tesla struct {
	Color string
}

func (t *Tesla) Drive() error { return nil }

// Custom has custom marshalers and unmarshalers, taking pointer receivers.
type CustomPtr struct {
	Value string
}

func (c *CustomPtr) MarshalJSON() ([]byte, error) {
	return []byte("\"custom\""), nil
}

func (c *CustomPtr) UnmarshalJSON(bz []byte) error {
	c.Value = "custom"
	return nil
}

// CustomValue has custom marshalers and unmarshalers, taking value receivers (which usually doesn't
// make much sense since the unmarshaler can't change anything).
type CustomValue struct {
	Value string
}

func (c CustomValue) MarshalJSON() ([]byte, error) {
	return []byte("\"custom\""), nil
}

func (c CustomValue) UnmarshalJSON(bz []byte) error {
	c.Value = "custom"
	return nil
}

// Tags tests JSON tags.
type Tags struct {
	JSONName  string `json:"name"`
	OmitEmpty string `json:",omitempty"`
	Tags      *Tags  `json:"tags,omitempty"`
}

// Struct tests structs with lots of contents.
type Struct struct {
	Bool         bool
	Float64      float64
	Int32        int32
	Int64        int64
	Int64Ptr     *int64
	String       string
	StringPtrPtr **string
	Bytes        []byte
	Time         time.Time
	Car          Car
	Child        *Struct
}
