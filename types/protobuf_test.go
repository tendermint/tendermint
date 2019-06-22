package types

import (
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/assert"

	amino "github.com/tendermint/go-amino"
	abci "github.com/tendermint/tendermint/abci/types"
	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/crypto/secp256k1"
	"github.com/tendermint/tendermint/version"
)

func TestABCIPubKey(t *testing.T) {
	pkEd := ed25519.GenPrivKey().PubKey()
	pkSecp := secp256k1.GenPrivKey().PubKey()
	testABCIPubKey(t, pkEd, ABCIPubKeyTypeEd25519)
	testABCIPubKey(t, pkSecp, ABCIPubKeyTypeSecp256k1)
}

func testABCIPubKey(t *testing.T, pk crypto.PubKey, typeStr string) {
	abciPubKey := TM2PB.PubKey(pk)
	pk2, err := PB2TM.PubKey(abciPubKey)
	assert.Nil(t, err)
	assert.Equal(t, pk, pk2)
}

func TestABCIValidators(t *testing.T) {
	pkEd := ed25519.GenPrivKey().PubKey()

	// correct validator
	tmValExpected := NewValidator(pkEd, 10)

	tmVal := NewValidator(pkEd, 10)

	abciVal := TM2PB.ValidatorUpdate(tmVal)
	tmVals, err := PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{abciVal})
	assert.Nil(t, err)
	assert.Equal(t, tmValExpected, tmVals[0])

	abciVals := TM2PB.ValidatorUpdates(NewValidatorSet(tmVals))
	assert.Equal(t, []abci.ValidatorUpdate{abciVal}, abciVals)

	// val with address
	tmVal.Address = pkEd.Address()

	abciVal = TM2PB.ValidatorUpdate(tmVal)
	tmVals, err = PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{abciVal})
	assert.Nil(t, err)
	assert.Equal(t, tmValExpected, tmVals[0])

	// val with incorrect pubkey data
	abciVal = TM2PB.ValidatorUpdate(tmVal)
	abciVal.PubKey.Data = []byte("incorrect!")
	tmVals, err = PB2TM.ValidatorUpdates([]abci.ValidatorUpdate{abciVal})
	assert.NotNil(t, err)
	assert.Nil(t, tmVals)
}

func TestABCIConsensusParams(t *testing.T) {
	cp := DefaultConsensusParams()
	abciCP := TM2PB.ConsensusParams(cp)
	cp2 := cp.Update(abciCP)

	assert.Equal(t, *cp, cp2)
}

func newHeader(
	height, numTxs int64,
	commitHash, dataHash, evidenceHash []byte,
) *Header {
	return &Header{
		Height:         height,
		NumTxs:         numTxs,
		LastCommitHash: commitHash,
		DataHash:       dataHash,
		EvidenceHash:   evidenceHash,
	}
}

func TestABCIHeader(t *testing.T) {
	// build a full header
	var height int64 = 5
	var numTxs int64 = 3
	header := newHeader(
		height, numTxs,
		[]byte("lastCommitHash"), []byte("dataHash"), []byte("evidenceHash"),
	)
	protocolVersion := version.Consensus{Block: 7, App: 8}
	timestamp := time.Now()
	lastBlockID := BlockID{
		Hash: []byte("hash"),
		PartsHeader: PartSetHeader{
			Total: 10,
			Hash:  []byte("hash"),
		},
	}
	var totalTxs int64 = 100
	header.Populate(
		protocolVersion, "chainID",
		timestamp, lastBlockID, totalTxs,
		[]byte("valHash"), []byte("nextValHash"),
		[]byte("consHash"), []byte("appHash"), []byte("lastResultsHash"),
		[]byte("proposerAddress"),
	)

	cdc := amino.NewCodec()
	headerBz := cdc.MustMarshalBinaryBare(header)

	pbHeader := TM2PB.Header(header)
	pbHeaderBz, err := proto.Marshal(&pbHeader)
	assert.NoError(t, err)

	// assert some fields match
	assert.EqualValues(t, protocolVersion.Block, pbHeader.Version.Block)
	assert.EqualValues(t, protocolVersion.App, pbHeader.Version.App)
	assert.EqualValues(t, "chainID", pbHeader.ChainID)
	assert.EqualValues(t, height, pbHeader.Height)
	assert.EqualValues(t, timestamp, pbHeader.Time)
	assert.EqualValues(t, numTxs, pbHeader.NumTxs)
	assert.EqualValues(t, totalTxs, pbHeader.TotalTxs)
	assert.EqualValues(t, lastBlockID.Hash, pbHeader.LastBlockId.Hash)
	assert.EqualValues(t, []byte("lastCommitHash"), pbHeader.LastCommitHash)
	assert.Equal(t, []byte("proposerAddress"), pbHeader.ProposerAddress)

	// assert the encodings match
	// NOTE: they don't yet because Amino encodes
	// int64 as zig-zag and we're using non-zigzag in the protobuf.
	// See https://github.com/tendermint/tendermint/issues/2682
	_, _ = headerBz, pbHeaderBz
	// assert.EqualValues(t, headerBz, pbHeaderBz)

}

func TestABCIEvidence(t *testing.T) {
	val := NewMockPV()
	blockID := makeBlockID([]byte("blockhash"), 1000, []byte("partshash"))
	blockID2 := makeBlockID([]byte("blockhash2"), 1000, []byte("partshash"))
	const chainID = "mychain"
	pubKey := val.GetPubKey()
	ev := &DuplicateVoteEvidence{
		PubKey: pubKey,
		VoteA:  makeVote(val, chainID, 0, 10, 2, 1, blockID),
		VoteB:  makeVote(val, chainID, 0, 10, 2, 1, blockID2),
	}
	abciEv := TM2PB.Evidence(
		ev,
		NewValidatorSet([]*Validator{NewValidator(pubKey, 10)}),
		time.Now(),
	)

	assert.Equal(t, "duplicate/vote", abciEv.Type)
}

type pubKeyEddie struct{}

func (pubKeyEddie) Address() Address                        { return []byte{} }
func (pubKeyEddie) Bytes() []byte                           { return []byte{} }
func (pubKeyEddie) VerifyBytes(msg []byte, sig []byte) bool { return false }
func (pubKeyEddie) Equals(crypto.PubKey) bool               { return false }

func TestABCIValidatorFromPubKeyAndPower(t *testing.T) {
	pubkey := ed25519.GenPrivKey().PubKey()

	abciVal := TM2PB.NewValidatorUpdate(pubkey, 10)
	assert.Equal(t, int64(10), abciVal.Power)

	assert.Panics(t, func() { TM2PB.NewValidatorUpdate(nil, 10) })
	assert.Panics(t, func() { TM2PB.NewValidatorUpdate(pubKeyEddie{}, 10) })
}

func TestABCIValidatorWithoutPubKey(t *testing.T) {
	pkEd := ed25519.GenPrivKey().PubKey()

	abciVal := TM2PB.Validator(NewValidator(pkEd, 10))

	// pubkey must be nil
	tmValExpected := abci.Validator{
		Address: pkEd.Address(),
		Power:   10,
	}

	assert.Equal(t, tmValExpected, abciVal)
}
