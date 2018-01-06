package expr

import (
	"errors"
	"strconv"

	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/go-wire"
)

type Byteful interface {
	Bytes() ([]byte, error)
}

//----------------------------------------

type Numeric struct {
	Type   string
	Number string
}

func (n Numeric) Bytes() ([]byte, error) {
	num, err := strconv.Atoi(n.Number)
	if err != nil {
		return nil, err
	}
	switch n.Type {
	case "u": // Uvarint
		return wire.BinaryBytes(uint(num)), nil
	case "i": // Varint
		return wire.BinaryBytes(int(num)), nil
	case "u64": // Uint64
		return wire.BinaryBytes(uint64(num)), nil
	case "i64": // Int64
		return wire.BinaryBytes(int64(num)), nil
	case "u32": // Uint32
		return wire.BinaryBytes(uint32(num)), nil
	case "i32": // Int32
		return wire.BinaryBytes(int32(num)), nil
	case "u16": // Uint16
		return wire.BinaryBytes(uint16(num)), nil
	case "i16": // Int16
		return wire.BinaryBytes(int16(num)), nil
	case "u8": // Uint8
		return wire.BinaryBytes(uint8(num)), nil
	case "i8": // Int8
		return wire.BinaryBytes(int8(num)), nil
	}
	return nil, errors.New(cmn.Fmt("Unknown Numeric type %v", n.Type))
}

//----------------------------------------

type Tuple []interface{}

func (t Tuple) Bytes() ([]byte, error) {
	bz := []byte{}
	for _, item := range t {
		if _, ok := item.(Byteful); !ok {
			return nil, errors.New("Tuple item was not Byteful")
		}
		bzi, err := item.(Byteful).Bytes()
		if err != nil {
			return nil, err
		}
		bz = append(bz, bzi...)
	}
	return bz, nil
}

func (t Tuple) String() string {
	s := "("
	for i, ti := range t {
		if i == 0 {
			s += cmn.Fmt("%v", ti)
		} else {
			s += cmn.Fmt(" %v", ti)
		}
	}
	s += ")"
	return s
}

//----------------------------------------

type Array []interface{}

func (arr Array) Bytes() ([]byte, error) {
	bz := wire.BinaryBytes(int(len(arr)))
	for _, item := range arr {
		if _, ok := item.(Byteful); !ok {
			return nil, errors.New("Array item was not Byteful")
		}
		bzi, err := item.(Byteful).Bytes()
		if err != nil {
			return nil, err
		}
		bz = append(bz, bzi...)
	}
	return bz, nil
}

func (t Array) String() string {
	s := "["
	for i, ti := range t {
		if i == 0 {
			s += cmn.Fmt("%v", ti)
		} else {
			s += cmn.Fmt(",%v", ti)
		}
	}
	s += "]"
	return s
}

//----------------------------------------

type Bytes struct {
	Data           []byte
	LengthPrefixed bool
}

func NewBytes(bz []byte, lengthPrefixed bool) Bytes {
	return Bytes{
		Data:           bz,
		LengthPrefixed: lengthPrefixed,
	}
}

func (b Bytes) Bytes() ([]byte, error) {
	if b.LengthPrefixed {
		bz := wire.BinaryBytes(len(b.Data))
		bz = append(bz, b.Data...)
		return bz, nil
	} else {
		return b.Data, nil
	}
}

func (b Bytes) String() string {
	if b.LengthPrefixed {
		return cmn.Fmt("0x%X", []byte(b.Data))
	} else {
		return cmn.Fmt("x%X", []byte(b.Data))
	}
}

//----------------------------------------

type Placeholder struct {
	Label string
}

func (p Placeholder) Bytes() ([]byte, error) {
	return []byte{0x00}, nil
}

func (p Placeholder) String() string {
	return cmn.Fmt("<%v>", p.Label)
}

//----------------------------------------

type String struct {
	Text string
}

func NewString(text string) String {
	return String{text}
}

func (s String) Bytes() ([]byte, error) {
	bz := wire.BinaryBytes(int(len(s.Text)))
	bz = append(bz, []byte(s.Text)...)
	return bz, nil
}

func (s String) String() string {
	return strconv.Quote(s.Text)
}
