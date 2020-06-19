// Protocol Buffers for Go with Gadgets
//
// Copyright (c) 2013, The GoGo Authors. All rights reserved.
// http://github.com/gogo/protobuf
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//     * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//     * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package protoio_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math/rand"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/test"

	"github.com/tendermint/tendermint/libs/protoio"
)

func iotest(writer protoio.WriteCloser, reader protoio.ReadCloser) error {
	varint := make([]byte, binary.MaxVarintLen64)
	size := 1000
	msgs := make([]*test.NinOptNative, size)
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := range msgs {
		msgs[i] = test.NewPopulatedNinOptNative(r, true)
		//issue 31
		if i == 5 {
			msgs[i] = &test.NinOptNative{}
		}
		//issue 31
		if i == 999 {
			msgs[i] = &test.NinOptNative{}
		}
		// FIXME Check size
		bz, err := proto.Marshal(msgs[i])
		if err != nil {
			return err
		}
		visize := binary.PutUvarint(varint, uint64(len(bz)))
		n, err := writer.WriteMsg(msgs[i])
		if err != nil {
			return err
		}
		if n != len(bz)+visize {
			return fmt.Errorf("WriteMsg() wrote %v bytes, expected %v", n, len(bz)+visize) // nolint
		}
	}
	if err := writer.Close(); err != nil {
		return err
	}
	i := 0
	for {
		msg := &test.NinOptNative{}
		if err := reader.ReadMsg(msg); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
		if err := msg.VerboseEqual(msgs[i]); err != nil {
			return err
		}
		i++
	}
	if i != size {
		panic("not enough messages read")
	}
	if err := reader.Close(); err != nil {
		return err
	}
	return nil
}

type buffer struct {
	*bytes.Buffer
	closed bool
}

func (b *buffer) Close() error {
	b.closed = true
	return nil
}

func newBuffer() *buffer {
	return &buffer{bytes.NewBuffer(nil), false}
}

func TestVarintNormal(t *testing.T) {
	buf := newBuffer()
	writer := protoio.NewDelimitedWriter(buf)
	reader := protoio.NewDelimitedReader(buf, 1024*1024)
	if err := iotest(writer, reader); err != nil {
		t.Error(err)
	}
	if !buf.closed {
		t.Fatalf("did not close buffer")
	}
}

func TestVarintNoClose(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	writer := protoio.NewDelimitedWriter(buf)
	reader := protoio.NewDelimitedReader(buf, 1024*1024)
	if err := iotest(writer, reader); err != nil {
		t.Error(err)
	}
}

//issue 32
func TestVarintMaxSize(t *testing.T) {
	buf := newBuffer()
	writer := protoio.NewDelimitedWriter(buf)
	reader := protoio.NewDelimitedReader(buf, 20)
	if err := iotest(writer, reader); err == nil {
		t.Error(err)
	} else {
		t.Logf("%s", err)
	}
}

func TestVarintError(t *testing.T) {
	buf := newBuffer()
	buf.Write([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f})
	reader := protoio.NewDelimitedReader(buf, 1024*1024)
	msg := &test.NinOptNative{}
	err := reader.ReadMsg(msg)
	if err == nil {
		t.Fatalf("Expected error")
	}
}
