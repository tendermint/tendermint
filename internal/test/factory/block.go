package factory

import (
	"time"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/tmhash"
	"github.com/tendermint/tendermint/pkg/metadata"
	"github.com/tendermint/tendermint/version"
)

const (
	DefaultTestChainID = "test-chain"
)

var (
	DefaultTestTime = time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
)

func MakeVersion() version.Consensus {
	return version.Consensus{
		Block: version.BlockProtocol,
		App:   1,
	}
}

func RandomAddress() []byte {
	return crypto.CRandBytes(crypto.AddressSize)
}

func RandomHash() []byte {
	return crypto.CRandBytes(tmhash.Size)
}

func MakeBlockID() metadata.BlockID {
	return MakeBlockIDWithHash(RandomHash())
}

func MakeBlockIDWithHash(hash []byte) metadata.BlockID {
	return metadata.BlockID{
		Hash: hash,
		PartSetHeader: metadata.PartSetHeader{
			Total: 100,
			Hash:  RandomHash(),
		},
	}
}

// MakeHeader fills the rest of the contents of the header such that it passes
// validate basic
func MakeHeader(h *metadata.Header) (*metadata.Header, error) {
	if h.Version.Block == 0 {
		h.Version.Block = version.BlockProtocol
	}
	if h.Height == 0 {
		h.Height = 1
	}
	if h.LastBlockID.IsZero() {
		h.LastBlockID = MakeBlockID()
	}
	if h.ChainID == "" {
		h.ChainID = DefaultTestChainID
	}
	if len(h.LastCommitHash) == 0 {
		h.LastCommitHash = RandomHash()
	}
	if len(h.DataHash) == 0 {
		h.DataHash = RandomHash()
	}
	if len(h.ValidatorsHash) == 0 {
		h.ValidatorsHash = RandomHash()
	}
	if len(h.NextValidatorsHash) == 0 {
		h.NextValidatorsHash = RandomHash()
	}
	if len(h.ConsensusHash) == 0 {
		h.ConsensusHash = RandomHash()
	}
	if len(h.AppHash) == 0 {
		h.AppHash = RandomHash()
	}
	if len(h.LastResultsHash) == 0 {
		h.LastResultsHash = RandomHash()
	}
	if len(h.EvidenceHash) == 0 {
		h.EvidenceHash = RandomHash()
	}
	if len(h.ProposerAddress) == 0 {
		h.ProposerAddress = RandomAddress()
	}

	return h, h.ValidateBasic()
}

func MakeRandomHeader() *metadata.Header {
	h, err := MakeHeader(&metadata.Header{})
	if err != nil {
		panic(err)
	}
	return h
}
