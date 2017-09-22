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

package types

import (
	"fmt"
	"io"

	"github.com/tendermint/go-crypto"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/go-wire/data"
	cmn "github.com/tendermint/tmlibs/common"
)

// Heartbeat is a simple vote-like structure so validators can alert others that
// they are alive and waiting for transactions.
type Heartbeat struct {
	ValidatorAddress data.Bytes       `json:"validator_address"`
	ValidatorIndex   int              `json:"validator_index"`
	Height           int              `json:"height"`
	Round            int              `json:"round"`
	Sequence         int              `json:"sequence"`
	Signature        crypto.Signature `json:"signature"`
}

// WriteSignBytes writes the Heartbeat for signing.
func (heartbeat *Heartbeat) WriteSignBytes(chainID string, w io.Writer, n *int, err *error) {
	wire.WriteJSON(CanonicalJSONOnceHeartbeat{
		chainID,
		CanonicalHeartbeat(heartbeat),
	}, w, n, err)
}

// Copy makes a copy of the Heartbeat.
func (heartbeat *Heartbeat) Copy() *Heartbeat {
	heartbeatCopy := *heartbeat
	return &heartbeatCopy
}

// String returns a string representation of the Heartbeat.
func (heartbeat *Heartbeat) String() string {
	if heartbeat == nil {
		return "nil-heartbeat"
	}

	return fmt.Sprintf("Heartbeat{%v:%X %v/%02d (%v) %v}",
		heartbeat.ValidatorIndex, cmn.Fingerprint(heartbeat.ValidatorAddress),
		heartbeat.Height, heartbeat.Round, heartbeat.Sequence, heartbeat.Signature)
}
