package blocks

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"testing"
	"time"

	"github.com/ugorji/go/codec"
	"github.com/vmihailenco/msgpack"
)

func BenchmarkTestCustom(b *testing.B) {
	b.StopTimer()

	h := &Header{
		Name:           "Header",
		Height:         123,
		Fees:           123,
		Time:           time.Unix(123, 0),
		PrevHash:       []byte("prevhash"),
		ValidationHash: []byte("validationhash"),
		DataHash:       []byte("datahash"),
	}

	buf := bytes.NewBuffer(nil)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		h.WriteTo(buf)
		var n int64
		var err error
		h2 := ReadHeader(buf, &n, &err)
		if h2.Name != "Header" {
			b.Fatalf("wrong name")
		}
	}
}

type HHeader struct {
	Name           string `json:"N"`
	Height         uint64 `json:"H"`
	Fees           uint64 `json:"F"`
	Time           uint64 `json:"T"`
	PrevHash       []byte `json:"PH"`
	ValidationHash []byte `json:"VH"`
	DataHash       []byte `json:"DH"`
}

func BenchmarkTestJSON(b *testing.B) {
	b.StopTimer()

	h := &HHeader{
		Name:           "Header",
		Height:         123,
		Fees:           123,
		Time:           123,
		PrevHash:       []byte("prevhash"),
		ValidationHash: []byte("validationhash"),
		DataHash:       []byte("datahash"),
	}
	h2 := &HHeader{}

	buf := bytes.NewBuffer(nil)
	enc := json.NewEncoder(buf)
	dec := json.NewDecoder(buf)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		enc.Encode(h)
		dec.Decode(h2)
		if h2.Name != "Header" {
			b.Fatalf("wrong name")
		}
	}
}

func BenchmarkTestGob(b *testing.B) {
	b.StopTimer()

	h := &Header{
		Name:           "Header",
		Height:         123,
		Fees:           123,
		Time:           time.Unix(123, 0),
		PrevHash:       []byte("prevhash"),
		ValidationHash: []byte("validationhash"),
		DataHash:       []byte("datahash"),
	}
	h2 := &Header{}

	buf := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(buf)
	dec := gob.NewDecoder(buf)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		enc.Encode(h)
		dec.Decode(h2)
		if h2.Name != "Header" {
			b.Fatalf("wrong name")
		}
	}
}

func BenchmarkTestMsgPack(b *testing.B) {
	b.StopTimer()

	h := &Header{
		Name:           "Header",
		Height:         123,
		Fees:           123,
		Time:           time.Unix(123, 0),
		PrevHash:       []byte("prevhash"),
		ValidationHash: []byte("validationhash"),
		DataHash:       []byte("datahash"),
	}
	h2 := &Header{}

	buf := bytes.NewBuffer(nil)
	enc := msgpack.NewEncoder(buf)
	dec := msgpack.NewDecoder(buf)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		enc.Encode(h)
		dec.Decode(h2)
		if h2.Name != "Header" {
			b.Fatalf("wrong name")
		}
	}
}

func BenchmarkTestMsgPack2(b *testing.B) {
	b.StopTimer()

	h := &Header{
		Name:           "Header",
		Height:         123,
		Fees:           123,
		Time:           time.Unix(123, 0),
		PrevHash:       []byte("prevhash"),
		ValidationHash: []byte("validationhash"),
		DataHash:       []byte("datahash"),
	}
	h2 := &Header{}
	var mh codec.MsgpackHandle
	handle := &mh

	buf := bytes.NewBuffer(nil)
	enc := codec.NewEncoder(buf, handle)
	dec := codec.NewDecoder(buf, handle)

	b.StartTimer()
	for i := 0; i < b.N; i++ {
		buf.Reset()
		enc.Encode(h)
		dec.Decode(h2)
		if h2.Name != "Header" {
			b.Fatalf("wrong name")
		}
	}
}
