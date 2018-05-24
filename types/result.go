package types

import common "github.com/tendermint/tmlibs/common"

// nondeterministic
type ResultException struct {
	Error string `json:"error,omitempty"`
}

type ResultEcho struct {
	Message string `json:"message,omitempty"`
}

type ResultFlush struct {
}

type ResultInfo struct {
	Data             string `json:"data,omitempty"`
	Version          string `json:"version,omitempty"`
	LastBlockHeight  int64  `json:"last_block_height,omitempty"`
	LastBlockAppHash []byte `json:"last_block_app_hash,omitempty"`
}

type ResultSetOption struct {
	Code uint32 `json:"code,omitempty"`
	// bytes data = 2;
	Log  string `json:"log,omitempty"`
	Info string `json:"info,omitempty"`
}

type ResultInitChain struct {
	Validators []Validator `json:"validators"`
}

type ResultQuery struct {
	Code uint32 `json:"code,omitempty"`
	// bytes data = 2; // use "value" instead.
	Log    string `json:"log,omitempty"`
	Info   string `json:"info,omitempty"`
	Index  int64  `json:"index,omitempty"`
	Key    []byte `json:"key,omitempty"`
	Value  []byte `json:"value,omitempty"`
	Proof  []byte `json:"proof,omitempty"`
	Height int64  `json:"height,omitempty"`
}

type ResultBeginBlock struct {
	Tags []common.KVPair `json:"tags,omitempty"`
}

type ResultCheckTx struct {
	Code      uint32          `json:"code,omitempty"`
	Data      []byte          `json:"data,omitempty"`
	Log       string          `json:"log,omitempty"`
	Info      string          `json:"info,omitempty"`
	GasWanted int64           `json:"gas_wanted,omitempty"`
	GasUsed   int64           `json:"gas_used,omitempty"`
	Tags      []common.KVPair `json:"tags,omitempty"`
	Fee       common.KI64Pair `json:"fee"`
}

type ResultDeliverTx struct {
	Code      uint32          `json:"code,omitempty"`
	Data      []byte          `json:"data,omitempty"`
	Log       string          `json:"log,omitempty"`
	Info      string          `json:"info,omitempty"`
	GasWanted int64           `json:"gas_wanted,omitempty"`
	GasUsed   int64           `json:"gas_used,omitempty"`
	Tags      []common.KVPair `json:"tags,omitempty"`
	Fee       common.KI64Pair `json:"fee"`
}

type ResultEndBlock struct {
	ValidatorUpdates      []Validator      `json:"validator_updates"`
	ConsensusParamUpdates *ConsensusParams `json:"consensus_param_updates,omitempty"`
	Tags                  []common.KVPair  `json:"tags,omitempty"`
}

type ResultCommit struct {
	// reserve 1
	Data []byte `json:"data,omitempty"`
}
