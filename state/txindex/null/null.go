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

package null

import (
	"errors"

	"github.com/tendermint/tendermint/state/txindex"
	"github.com/tendermint/tendermint/types"
)

// TxIndex acts as a /dev/null.
type TxIndex struct{}

// Tx panics.
func (txi *TxIndex) Get(hash []byte) (*types.TxResult, error) {
	return nil, errors.New(`Indexing is disabled (set 'tx_index = "kv"' in config)`)
}

// Batch returns nil.
func (txi *TxIndex) AddBatch(batch *txindex.Batch) error {
	return nil
}
