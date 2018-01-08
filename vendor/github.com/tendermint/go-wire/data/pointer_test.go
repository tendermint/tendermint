package data_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	data "github.com/tendermint/go-wire/data"
)

type Ptr interface {
	Point() string
}

// implementations
type PStr string
type PInt int

func (p *PStr) Point() string {
	if p == nil {
		return "snil"
	}
	return string(*p)
}

func (p *PInt) Point() string {
	if p == nil {
		return "inil"
	}
	return fmt.Sprintf("int: %d", int(*p))
}

var ptrMapper data.Mapper

// we register pointers to those structs, as they fulfill the interface
// but mainly so we can test how they handle nil values in the struct
func init() {
	ps, pi := PStr(""), PInt(0)
	ptrMapper = data.NewMapper(KeyS{}).
		RegisterImplementation(&ps, "str", 5).
		RegisterImplementation(&pi, "int", 25)
}

// PtrS adds json serialization to Ptr
type PtrS struct {
	Ptr
}

func (p PtrS) MarshalJSON() ([]byte, error) {
	return ptrMapper.ToJSON(p.Ptr)
}

func (p *PtrS) UnmarshalJSON(data []byte) (err error) {
	parsed, err := ptrMapper.FromJSON(data)
	if err == nil && parsed != nil {
		p.Ptr = parsed.(Ptr)
	}
	return
}

// TestEncodingNil happens when we have a nil inside the embedding struct
func TestEncodingNil(t *testing.T) {
	assert, require := assert.New(t), require.New(t)

	s := PStr("foo")
	i := PInt(567)
	ptrs := []Ptr{&s, &i, nil}

	for i, p := range ptrs {
		wrap := PtrS{p}

		js, err := data.ToJSON(wrap)
		require.Nil(err, "%d: %+v", i, err)

		parsed := PtrS{}
		err = data.FromJSON(js, &parsed)
		require.Nil(err, "%d: %+v", i, err)
		if wrap.Ptr != nil {
			assert.Equal(parsed.Point(), wrap.Point())
		}
		assert.Equal(parsed.Ptr, wrap.Ptr)
	}
}
