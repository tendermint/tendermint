package base58

import (
	"encoding/json"

	"github.com/pkg/errors"
	data "github.com/tendermint/go-wire/data"
)

var (
	BTCEncoder    data.ByteEncoder = base58Encoder{BTCAlphabet}
	FlickrEncoder                  = base58Encoder{FlickrAlphabet}
)

// base58Encoder implements ByteEncoder encoding the slice as
// base58 url-safe encoding
type base58Encoder struct {
	alphabet string
}

func (e base58Encoder) _assertByteEncoder() data.ByteEncoder {
	return e
}

func (e base58Encoder) Unmarshal(dst *[]byte, src []byte) (err error) {
	var s string
	err = json.Unmarshal(src, &s)
	if err != nil {
		return errors.Wrap(err, "parse string")
	}
	*dst, err = DecodeAlphabet(s, e.alphabet)
	return err
}

func (e base58Encoder) Marshal(bytes []byte) ([]byte, error) {
	s := EncodeAlphabet(bytes, e.alphabet)
	return json.Marshal(s)
}
