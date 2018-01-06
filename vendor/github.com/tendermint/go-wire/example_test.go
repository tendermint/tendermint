// Copyright 2017 Tendermint. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package wire_test

import (
	"bytes"
	"fmt"
	"log"

	"github.com/tendermint/go-wire"
)

func Example_RegisterInterface() {
	type Receiver interface{}
	type bcMessage struct {
		Message string
		Height  int
	}

	type bcResponse struct {
		Status  int
		Message string
	}

	type bcStatus struct {
		Peers int
	}

	var _ = wire.RegisterInterface(
		struct{ Receiver }{},
		wire.ConcreteType{&bcMessage{}, 0x01},
		wire.ConcreteType{&bcResponse{}, 0x02},
		wire.ConcreteType{&bcStatus{}, 0x03},
	)
}

func Example_EndToEnd_ReadWriteBinary() {
	type Receiver interface{}
	type bcMessage struct {
		Message string
		Height  int
	}

	type bcResponse struct {
		Status  int
		Message string
	}

	type bcStatus struct {
		Peers int
	}

	var _ = wire.RegisterInterface(
		struct{ Receiver }{},
		wire.ConcreteType{&bcMessage{}, 0x01},
		wire.ConcreteType{&bcResponse{}, 0x02},
		wire.ConcreteType{&bcStatus{}, 0x03},
	)

	var n int
	var err error
	buf := new(bytes.Buffer)
	bm := &bcMessage{Message: "Tendermint", Height: 100}
	wire.WriteBinary(bm, buf, &n, &err)
	if err != nil {
		log.Fatalf("writeBinary: %v", err)
	}
	fmt.Printf("Encoded: %x\n", buf.Bytes())

	recv := wire.ReadBinary(struct{ Receiver }{}, buf, 0, &n, &err).(struct{ Receiver }).Receiver
	if err != nil {
		log.Fatalf("readBinary: %v", err)
	}
	decoded := recv.(*bcMessage)
	fmt.Printf("Decoded: %#v\n", decoded)

	// Output:
	// Encoded: 01010a54656e6465726d696e740164
	// Decoded: &wire_test.bcMessage{Message:"Tendermint", Height:100}
}
