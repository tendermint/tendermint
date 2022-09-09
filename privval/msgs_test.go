package privval

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/bls12381"
	"github.com/tendermint/tendermint/crypto/encoding"
	cryptoproto "github.com/tendermint/tendermint/proto/tendermint/crypto"
	privproto "github.com/tendermint/tendermint/proto/tendermint/privval"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

var stamp = time.Date(2019, 10, 13, 16, 14, 44, 0, time.UTC)

func exampleVote() *types.Vote {
	return &types.Vote{
		Type:   tmproto.PrecommitType,
		Height: 3,
		Round:  2,
		BlockID: types.BlockID{
			Hash: crypto.Checksum([]byte("blockID_hash")),
			PartSetHeader: types.PartSetHeader{
				Total: 1000000,
				Hash:  crypto.Checksum([]byte("blockID_part_set_header_hash")),
			},
		},
		ValidatorProTxHash: crypto.ProTxHashFromSeedBytes([]byte("validator_pro_tx_hash")),
		ValidatorIndex:     56789,
		VoteExtensions: types.VoteExtensions{
			tmproto.VoteExtensionType_DEFAULT: []types.VoteExtension{{Extension: []byte("extension")}},
		},
	}
}

func exampleProposal() *types.Proposal {

	return &types.Proposal{
		Type:      tmproto.SignedMsgType(1),
		Height:    3,
		Round:     2,
		Timestamp: stamp,
		POLRound:  2,
		Signature: []byte("it's a signature"),
		BlockID: types.BlockID{
			Hash: crypto.Checksum([]byte("blockID_hash")),
			PartSetHeader: types.PartSetHeader{
				Total: 1000000,
				Hash:  crypto.Checksum([]byte("blockID_part_set_header_hash")),
			},
		},
	}
}

func TestPrivvalVectors(t *testing.T) {
	pk := bls12381.GenPrivKeyFromSecret([]byte("it's a secret")).PubKey()
	ppk, err := encoding.PubKeyToProto(pk)
	require.NoError(t, err)

	// Generate a simple vote
	vote := exampleVote()
	votepb := vote.ToProto()

	// Generate a simple proposal
	proposal := exampleProposal()
	proposalpb := proposal.ToProto()

	// State ID
	stateID := vote.StateID()
	stateIDpb := stateID.ToProto()

	// Create a Reuseable remote error
	remoteError := &privproto.RemoteSignerError{Code: 1, Description: "it's a error"}

	testCases := []struct {
		testName string
		msg      proto.Message
		expBytes string
	}{
		{"ping request", &privproto.PingRequest{}, "3a00"},
		{"ping response", &privproto.PingResponse{}, "4200"},
		{"pubKey request", &privproto.PubKeyRequest{}, "0a00"},
		{"pubKey response", &privproto.PubKeyResponse{PubKey: ppk, Error: nil}, "12340a32223011c7f5ac5a6d01fd9dde3840f7ebbb6a20deed6fba72a347dd66da2f8c9c977c6604b2cd2e0148206c2add9a8f5ddd74"},
		{"pubKey response with error", &privproto.PubKeyResponse{PubKey: cryptoproto.PublicKey{}, Error: remoteError}, "12140a0012100801120c697427732061206572726f72"},
		{"Vote Request", &privproto.SignVoteRequest{Vote: votepb, StateId: &stateIDpb}, "1a8c010a8501080210031802224a0a208b01023386c371778ecb6368573e539afc3cc860ec3a2f614e54fe5652f4fc80122608c0843d122072db3d959635dff1bb567bedaa70573392c5159666a3f8caf11e413aac52207a3220959a8f5ef2be68d0ed3a07ed8cff85991ee7995c2ac17030f742c135f9729fbe38d5bb035a0b1209657874656e73696f6e2a021003"},
		{"Vote Response", &privproto.SignedVoteResponse{Vote: *votepb, Error: nil}, "2288010a8501080210031802224a0a208b01023386c371778ecb6368573e539afc3cc860ec3a2f614e54fe5652f4fc80122608c0843d122072db3d959635dff1bb567bedaa70573392c5159666a3f8caf11e413aac52207a3220959a8f5ef2be68d0ed3a07ed8cff85991ee7995c2ac17030f742c135f9729fbe38d5bb035a0b1209657874656e73696f6e"},
		{"Vote Response with error", &privproto.SignedVoteResponse{Vote: tmproto.Vote{}, Error: remoteError}, "22180a042202120012100801120c697427732061206572726f72"},
		{"Proposal Request", &privproto.SignProposalRequest{Proposal: proposalpb}, "2a700a6e08011003180220022a4a0a208b01023386c371778ecb6368573e539afc3cc860ec3a2f614e54fe5652f4fc80122608c0843d122072db3d959635dff1bb567bedaa70573392c5159666a3f8caf11e413aac52207a320608f49a8ded053a10697427732061207369676e6174757265"},
		{"Proposal Response", &privproto.SignedProposalResponse{Proposal: *proposalpb, Error: nil}, "32700a6e08011003180220022a4a0a208b01023386c371778ecb6368573e539afc3cc860ec3a2f614e54fe5652f4fc80122608c0843d122072db3d959635dff1bb567bedaa70573392c5159666a3f8caf11e413aac52207a320608f49a8ded053a10697427732061207369676e6174757265"},
		{"Proposal Response with error", &privproto.SignedProposalResponse{Proposal: tmproto.Proposal{}, Error: remoteError}, "32250a112a021200320b088092b8c398feffffff0112100801120c697427732061206572726f72"},
	}

	for _, tc := range testCases {
		t.Run(tc.testName, func(t *testing.T) {
			pm := mustWrapMsg(tc.msg)
			bz, err := pm.Marshal()
			require.NoError(t, err, tc.testName)

			require.Equal(t, tc.expBytes, hex.EncodeToString(bz), tc.testName)
		})
	}
}
