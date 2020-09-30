package rpc

import (
	"github.com/tendermint/tendermint/crypto/merkle"
)

func defaultProofRuntime() *merkle.ProofRuntime {
	prt := merkle.NewProofRuntime()
	prt.RegisterOpDecoder(
		merkle.ProofOpValue,
		merkle.ValueOpDecoder,
	)
	return prt
}
