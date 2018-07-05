package proxy

import (
	"github.com/tendermint/iavl"
	"github.com/tendermint/tendermint/crypto/merkle"
)

func defaultProofRuntime() *merkle.ProofRuntime {
	prt := merkle.NewProofRuntime()
	prt.RegisterOpDecoder(
		merkle.ProofOpSimpleValue,
		merkle.SimpleValueOpDecoder,
	)
	prt.RegisterOpDecoder(
		iavl.ProofOpIAVLValue,
		iavl.IAVLValueOpDecoder,
	)
	prt.RegisterOpDecoder(
		iavl.ProofOpIAVLAbsence,
		iavl.IAVLAbsenceOpDecoder,
	)
	return prt
}
