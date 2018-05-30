package types

import cmn "github.com/tendermint/tmlibs/common"

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

func FromResultInfo(res ResultInfo) ResponseInfo {
	return ResponseInfo(res)
}

type ResultSetOption struct {
	Code uint32 `json:"code,omitempty"`
	// bytes data = 2;
	Log  string `json:"log,omitempty"`
	Info string `json:"info,omitempty"`
}

func FromResultSetOption(res ResultSetOption) ResponseSetOption {
	return ResponseSetOption(res)
}

type ResultInitChain struct {
	Validators []Validator `json:"validators"`
}

func FromResultInitChain(res ResultInitChain) ResponseInitChain {
	vals := valsToPointers(res.Validators)
	return ResponseInitChain{
		Validators: vals,
	}
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

func FromResultQuery(res ResultQuery) ResponseQuery {
	return ResponseQuery(res)
}

type ResultBeginBlock struct {
	Tags []cmn.KVPair `json:"tags,omitempty"`
}

func FromResultBeginBlock(res ResultBeginBlock) ResponseBeginBlock {
	tags := tagsToPointers(res.Tags)
	return ResponseBeginBlock{
		Tags: tags,
	}
}

type ResultCheckTx struct {
	Code      uint32       `json:"code,omitempty"`
	Data      []byte       `json:"data,omitempty"`
	Log       string       `json:"log,omitempty"`
	Info      string       `json:"info,omitempty"`
	GasWanted int64        `json:"gas_wanted,omitempty"`
	GasUsed   int64        `json:"gas_used,omitempty"`
	Tags      []cmn.KVPair `json:"tags,omitempty"`
	Fee       cmn.KI64Pair `json:"fee"`
}

func FromResultCheckTx(res ResultCheckTx) ResponseCheckTx {
	tags := tagsToPointers(res.Tags)
	return ResponseCheckTx{
		Code:      res.Code,
		Data:      res.Data,
		Log:       res.Log,
		Info:      res.Info,
		GasWanted: res.GasWanted,
		GasUsed:   res.GasUsed,
		Tags:      tags,
		Fee:       &res.Fee,
	}
}

type ResultDeliverTx struct {
	Code      uint32       `json:"code,omitempty"`
	Data      []byte       `json:"data,omitempty"`
	Log       string       `json:"log,omitempty"`
	Info      string       `json:"info,omitempty"`
	GasWanted int64        `json:"gas_wanted,omitempty"`
	GasUsed   int64        `json:"gas_used,omitempty"`
	Tags      []cmn.KVPair `json:"tags,omitempty"`
	Fee       cmn.KI64Pair `json:"fee"`
}

func FromResultDeliverTx(res ResultDeliverTx) ResponseDeliverTx {
	tags := tagsToPointers(res.Tags)
	return ResponseDeliverTx{
		Code:      res.Code,
		Data:      res.Data,
		Log:       res.Log,
		Info:      res.Info,
		GasWanted: res.GasWanted,
		GasUsed:   res.GasUsed,
		Tags:      tags,
		Fee:       &res.Fee,
	}
}

type ResultEndBlock struct {
	ValidatorUpdates      []Validator      `json:"validator_updates"`
	ConsensusParamUpdates *ConsensusParams `json:"consensus_param_updates,omitempty"`
	Tags                  []cmn.KVPair     `json:"tags,omitempty"`
}

func FromResultEndBlock(res ResultEndBlock) ResponseEndBlock {
	tags := tagsToPointers(res.Tags)
	vals := valsToPointers(res.ValidatorUpdates)
	return ResponseEndBlock{
		ValidatorUpdates:      vals,
		ConsensusParamUpdates: res.ConsensusParamUpdates,
		Tags: tags,
	}
}

type ResultCommit struct {
	// reserve 1
	Data []byte `json:"data,omitempty"`
}

func FromResultCommit(res ResultCommit) ResponseCommit {
	return ResponseCommit(res)
}

//-------------------------------------------------------

func tagsToPointers(tags []cmn.KVPair) []*cmn.KVPair {
	tagPtrs := make([]*cmn.KVPair, len(tags))
	for i := 0; i < len(tags); i++ {
		t := tags[i]
		tagPtrs[i] = &t
	}
	return tagPtrs
}

func valsToPointers(vals []Validator) []*Validator {
	valPtrs := make([]*Validator, len(vals))
	for i := 0; i < len(vals); i++ {
		v := vals[i]
		valPtrs[i] = &v
	}
	return valPtrs
}
