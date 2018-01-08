package data

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
)

// Encoder is a global setting for all byte encoding
// This is the default.  Please override in the main()/init()
// of your program to change how byte slices are presented
//
// In addition to these implementation, you can also find
// BTCEncoder and FlickrEncoder that use base58 variants in
// github.com/tendermint/go-wire/data/base58
var (
	Encoder       ByteEncoder = hexEncoder{}
	HexEncoder                = hexEncoder{}
	B64Encoder                = base64Encoder{base64.URLEncoding}
	RawB64Encoder             = base64Encoder{base64.RawURLEncoding}
)

// Bytes is a special byte slice that allows us to control the
// serialization format per app.
//
// Thus, basecoin could use hex, another app base64, and a third
// app base58...
type Bytes []byte

// Marshal needed for protobuf compatibility
func (b Bytes) Marshal() ([]byte, error) {
	return b, nil
}

// Unmarshal needed for protobuf compatibility
func (b *Bytes) Unmarshal(data []byte) error {
	*b = data
	return nil
}

func (b Bytes) MarshalJSON() ([]byte, error) {
	return Encoder.Marshal(b)
}

func (b *Bytes) UnmarshalJSON(data []byte) error {
	ref := (*[]byte)(b)
	return Encoder.Unmarshal(ref, data)
}

// Allow it to fulfill various interfaces in light-client, etc...
func (b Bytes) Bytes() []byte {
	return b
}

// String gets a simple string for printing (the json output minus quotes)
func (b Bytes) String() string {
	raw, err := Encoder.Marshal(b)
	l := len(raw)
	if err != nil || l < 2 {
		return "Bytes<?????>"
	}
	return string(raw[1 : l-1])
}

// ByteEncoder handles both the marshalling and unmarshalling of
// an arbitrary byte slice.
//
// All Bytes use the global Encoder set in this package.
// If you want to use this encoding for byte arrays, you can just
// implement a simple custom marshaller for your byte array
//
//   type Dings [64]byte
//
//   func (d Dings) MarshalJSON() ([]byte, error) {
//     return data.Encoder.Marshal(d[:])
//   }
//
//   func (d *Dings) UnmarshalJSON(enc []byte) error {
//     var ref []byte
//     err := data.Encoder.Unmarshal(&ref, enc)
//     copy(d[:], ref)
//     return err
//   }
type ByteEncoder interface {
	Marshal(bytes []byte) ([]byte, error)
	Unmarshal(dst *[]byte, src []byte) error
}

// hexEncoder implements ByteEncoder encoding the slice as a hexidecimal
// string
type hexEncoder struct{}

var _ ByteEncoder = hexEncoder{}

func (_ hexEncoder) Unmarshal(dst *[]byte, src []byte) (err error) {
	var s string
	err = json.Unmarshal(src, &s)
	if err != nil {
		return errors.Wrap(err, "parse string")
	}
	// and interpret that string as hex
	*dst, err = hex.DecodeString(s)
	return err
}

func (_ hexEncoder) Marshal(bytes []byte) ([]byte, error) {
	s := strings.ToUpper(hex.EncodeToString(bytes))
	return json.Marshal(s)
}

// base64Encoder implements ByteEncoder encoding the slice as
// base64 url-safe encoding
type base64Encoder struct {
	*base64.Encoding
}

var _ ByteEncoder = base64Encoder{}

func (e base64Encoder) Unmarshal(dst *[]byte, src []byte) (err error) {
	var s string
	err = json.Unmarshal(src, &s)
	if err != nil {
		return errors.Wrap(err, "parse string")
	}
	*dst, err = e.DecodeString(s)
	return err
}

func (e base64Encoder) Marshal(bytes []byte) ([]byte, error) {
	s := e.EncodeToString(bytes)
	return json.Marshal(s)
}
