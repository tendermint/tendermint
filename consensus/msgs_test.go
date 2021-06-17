package consensus

import (
	"encoding/hex"
	"math"
	"testing"
	"time"

	"github.com/tendermint/tendermint/crypto"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto/merkle"
	"github.com/tendermint/tendermint/libs/bits"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	"github.com/tendermint/tendermint/p2p"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func TestMsgToProto(t *testing.T) {
	psh := types.PartSetHeader{
		Total: 1,
		Hash:  tmrand.Bytes(32),
	}
	pbPsh := psh.ToProto()
	bi := types.BlockID{
		Hash:          tmrand.Bytes(32),
		PartSetHeader: psh,
	}
	pbBi := bi.ToProto()
	bits := bits.NewBitArray(1)
	pbBits := bits.ToProto()

	parts := types.Part{
		Index: 1,
		Bytes: []byte("test"),
		Proof: merkle.Proof{
			Total:    1,
			Index:    1,
			LeafHash: tmrand.Bytes(32),
			Aunts:    [][]byte{},
		},
	}
	pbParts, err := parts.ToProto()
	require.NoError(t, err)

	proposal := types.Proposal{
		Type:                  tmproto.ProposalType,
		Height:                1,
		CoreChainLockedHeight: 1,
		Round:                 1,
		POLRound:              1,
		BlockID:               bi,
		Timestamp:             time.Now(),
		Signature:             tmrand.Bytes(20),
	}
	pbProposal := proposal.ToProto()

	pv := types.NewMockPV()
	pk, err := pv.GetPubKey(crypto.QuorumHash{})
	require.NoError(t, err)
	val := types.NewValidatorDefaultVotingPower(pk, pv.ProTxHash)

	vote, err := types.MakeVote(
		1, types.BlockID{}, types.StateID{}, &types.ValidatorSet{Proposer: val, Validators: []*types.Validator{val}},
		pv, "chainID")
	require.NoError(t, err)
	pbVote := vote.ToProto()

	testsCases := []struct {
		testName string
		msg      Message
		want     *tmcons.Message
		wantErr  bool
	}{
		{"successful NewRoundStepMessage", &NewRoundStepMessage{
			Height:                2,
			Round:                 1,
			Step:                  1,
			SecondsSinceStartTime: 1,
			LastCommitRound:       2,
		}, &tmcons.Message{
			Sum: &tmcons.Message_NewRoundStep{
				NewRoundStep: &tmcons.NewRoundStep{
					Height:                2,
					Round:                 1,
					Step:                  1,
					SecondsSinceStartTime: 1,
					LastCommitRound:       2,
				},
			},
		}, false},

		{"successful NewValidBlockMessage", &NewValidBlockMessage{
			Height:             1,
			Round:              1,
			BlockPartSetHeader: psh,
			BlockParts:         bits,
			IsCommit:           false,
		}, &tmcons.Message{
			Sum: &tmcons.Message_NewValidBlock{
				NewValidBlock: &tmcons.NewValidBlock{
					Height:             1,
					Round:              1,
					BlockPartSetHeader: pbPsh,
					BlockParts:         pbBits,
					IsCommit:           false,
				},
			},
		}, false},
		{"successful BlockPartMessage", &BlockPartMessage{
			Height: 100,
			Round:  1,
			Part:   &parts,
		}, &tmcons.Message{
			Sum: &tmcons.Message_BlockPart{
				BlockPart: &tmcons.BlockPart{
					Height: 100,
					Round:  1,
					Part:   *pbParts,
				},
			},
		}, false},
		{"successful ProposalPOLMessage", &ProposalPOLMessage{
			Height:           1,
			ProposalPOLRound: 1,
			ProposalPOL:      bits,
		}, &tmcons.Message{
			Sum: &tmcons.Message_ProposalPol{
				ProposalPol: &tmcons.ProposalPOL{
					Height:           1,
					ProposalPolRound: 1,
					ProposalPol:      *pbBits,
				},
			}}, false},
		{"successful ProposalMessage", &ProposalMessage{
			Proposal: &proposal,
		}, &tmcons.Message{
			Sum: &tmcons.Message_Proposal{
				Proposal: &tmcons.Proposal{
					Proposal: *pbProposal,
				},
			},
		}, false},
		{"successful VoteMessage", &VoteMessage{
			Vote: vote,
		}, &tmcons.Message{
			Sum: &tmcons.Message_Vote{
				Vote: &tmcons.Vote{
					Vote: pbVote,
				},
			},
		}, false},
		{"successful VoteSetMaj23", &VoteSetMaj23Message{
			Height:  1,
			Round:   1,
			Type:    1,
			BlockID: bi,
		}, &tmcons.Message{
			Sum: &tmcons.Message_VoteSetMaj23{
				VoteSetMaj23: &tmcons.VoteSetMaj23{
					Height:  1,
					Round:   1,
					Type:    1,
					BlockID: pbBi,
				},
			},
		}, false},
		{"successful VoteSetBits", &VoteSetBitsMessage{
			Height:  1,
			Round:   1,
			Type:    1,
			BlockID: bi,
			Votes:   bits,
		}, &tmcons.Message{
			Sum: &tmcons.Message_VoteSetBits{
				VoteSetBits: &tmcons.VoteSetBits{
					Height:  1,
					Round:   1,
					Type:    1,
					BlockID: pbBi,
					Votes:   *pbBits,
				},
			},
		}, false},
		{"failure", nil, &tmcons.Message{}, true},
	}
	for _, tt := range testsCases {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			pb, err := MsgToProto(tt.msg)
			if tt.wantErr == true {
				assert.Equal(t, err != nil, tt.wantErr)
				return
			}
			assert.EqualValues(t, tt.want, pb, tt.testName)

			msg, err := MsgFromProto(pb)

			if !tt.wantErr {
				require.NoError(t, err)
				bcm := assert.Equal(t, tt.msg, msg, tt.testName)
				assert.True(t, bcm, tt.testName)
			} else {
				require.Error(t, err, tt.testName)
			}
		})
	}
}

func TestWALMsgProto(t *testing.T) {

	parts := types.Part{
		Index: 1,
		Bytes: []byte("test"),
		Proof: merkle.Proof{
			Total:    1,
			Index:    1,
			LeafHash: tmrand.Bytes(32),
			Aunts:    [][]byte{},
		},
	}
	pbParts, err := parts.ToProto()
	require.NoError(t, err)

	testsCases := []struct {
		testName string
		msg      WALMessage
		want     *tmcons.WALMessage
		wantErr  bool
	}{
		{"successful EventDataRoundState", types.EventDataRoundState{
			Height: 2,
			Round:  1,
			Step:   "ronies",
		}, &tmcons.WALMessage{
			Sum: &tmcons.WALMessage_EventDataRoundState{
				EventDataRoundState: &tmproto.EventDataRoundState{
					Height: 2,
					Round:  1,
					Step:   "ronies",
				},
			},
		}, false},
		{"successful msgInfo", msgInfo{
			Msg: &BlockPartMessage{
				Height: 100,
				Round:  1,
				Part:   &parts,
			},
			PeerID: p2p.ID("string"),
		}, &tmcons.WALMessage{
			Sum: &tmcons.WALMessage_MsgInfo{
				MsgInfo: &tmcons.MsgInfo{
					Msg: tmcons.Message{
						Sum: &tmcons.Message_BlockPart{
							BlockPart: &tmcons.BlockPart{
								Height: 100,
								Round:  1,
								Part:   *pbParts,
							},
						},
					},
					PeerID: "string",
				},
			},
		}, false},
		{"successful timeoutInfo", timeoutInfo{
			Duration: time.Duration(100),
			Height:   1,
			Round:    1,
			Step:     1,
		}, &tmcons.WALMessage{
			Sum: &tmcons.WALMessage_TimeoutInfo{
				TimeoutInfo: &tmcons.TimeoutInfo{
					Duration: time.Duration(100),
					Height:   1,
					Round:    1,
					Step:     1,
				},
			},
		}, false},
		{"successful EndHeightMessage", EndHeightMessage{
			Height: 1,
		}, &tmcons.WALMessage{
			Sum: &tmcons.WALMessage_EndHeight{
				EndHeight: &tmcons.EndHeight{
					Height: 1,
				},
			},
		}, false},
		{"failure", nil, &tmcons.WALMessage{}, true},
	}
	for _, tt := range testsCases {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			pb, err := WALToProto(tt.msg)
			if tt.wantErr == true {
				assert.Equal(t, err != nil, tt.wantErr)
				return
			}
			assert.EqualValues(t, tt.want, pb, tt.testName)

			msg, err := WALFromProto(pb)

			if !tt.wantErr {
				require.NoError(t, err)
				assert.Equal(t, tt.msg, msg, tt.testName) // need the concrete type as WAL Message is a empty interface
			} else {
				require.Error(t, err, tt.testName)
			}
		})
	}
}

// nolint:lll //ignore line length for tests
func TestConsMsgsVectors(t *testing.T) {
	date := time.Date(2018, 8, 30, 12, 0, 0, 0, time.UTC)
	psh := types.PartSetHeader{
		Total: 1,
		Hash:  []byte("add_more_exclamation_marks_code-"),
	}
	pbPsh := psh.ToProto()

	bi := types.BlockID{
		Hash:          []byte("add_more_exclamation_marks_code-"),
		PartSetHeader: psh,
	}

	si := types.StateID{
		LastAppHash: make([]byte, 32),
	}

	pbBi := bi.ToProto()
	bits := bits.NewBitArray(1)
	pbBits := bits.ToProto()

	parts := types.Part{
		Index: 1,
		Bytes: []byte("test"),
		Proof: merkle.Proof{
			Total:    1,
			Index:    1,
			LeafHash: []byte("add_more_exclamation_marks_code-"),
			Aunts:    [][]byte{},
		},
	}
	pbParts, err := parts.ToProto()
	require.NoError(t, err)

	proposal := types.Proposal{
		Type:                  tmproto.ProposalType,
		Height:                1,
		CoreChainLockedHeight: 1,
		Round:                 1,
		POLRound:              1,
		BlockID:               bi,
		Timestamp:             date,
		Signature:             []byte("add_more_exclamation"),
	}
	pbProposal := proposal.ToProto()

	v := &types.Vote{
		ValidatorProTxHash: []byte("add_more_exclamation"),
		ValidatorIndex:     1,
		Height:             1,
		Round:              0,
		Type:               tmproto.PrecommitType,
		BlockID:            bi,
		StateID:            si,
	}
	vpb := v.ToProto()

	testCases := []struct {
		testName string
		cMsg     proto.Message
		expBytes string
	}{
		{"NewRoundStep", &tmcons.Message{Sum: &tmcons.Message_NewRoundStep{NewRoundStep: &tmcons.NewRoundStep{
			Height:                1,
			Round:                 1,
			Step:                  1,
			SecondsSinceStartTime: 1,
			LastCommitRound:       1,
		}}}, "0a0a08011001180120012801"},
		{"NewRoundStep Max", &tmcons.Message{Sum: &tmcons.Message_NewRoundStep{NewRoundStep: &tmcons.NewRoundStep{
			Height:                math.MaxInt64,
			Round:                 math.MaxInt32,
			Step:                  math.MaxUint32,
			SecondsSinceStartTime: math.MaxInt64,
			LastCommitRound:       math.MaxInt32,
		}}}, "0a2608ffffffffffffffff7f10ffffffff0718ffffffff0f20ffffffffffffffff7f28ffffffff07"},
		{"NewValidBlock", &tmcons.Message{Sum: &tmcons.Message_NewValidBlock{
			NewValidBlock: &tmcons.NewValidBlock{
				Height: 1, Round: 1, BlockPartSetHeader: pbPsh, BlockParts: pbBits, IsCommit: false}}},
			"1231080110011a24080112206164645f6d6f72655f6578636c616d6174696f6e5f6d61726b735f636f64652d22050801120100"},
		{"Proposal", &tmcons.Message{Sum: &tmcons.Message_Proposal{Proposal: &tmcons.Proposal{Proposal: *pbProposal}}},
			"1a750a7308201001180120012a480a206164645f6d6f72655f6578636c616d6174696f6e5f6d61726b735f636f64652d1224080112206164645f6d6f72655f6578636c616d6174696f6e5f6d61726b735f636f64652d320608c0b89fdc053a146164645f6d6f72655f6578636c616d6174696f6ea00601"},
		{"ProposalPol", &tmcons.Message{Sum: &tmcons.Message_ProposalPol{
			ProposalPol: &tmcons.ProposalPOL{Height: 1, ProposalPolRound: 1}}},
			"2206080110011a00"},
		{"BlockPart", &tmcons.Message{Sum: &tmcons.Message_BlockPart{
			BlockPart: &tmcons.BlockPart{Height: 1, Round: 1, Part: *pbParts}}},
			"2a36080110011a3008011204746573741a26080110011a206164645f6d6f72655f6578636c616d6174696f6e5f6d61726b735f636f64652d"},
		{"Vote", &tmcons.Message{Sum: &tmcons.Message_Vote{
			Vote: &tmcons.Vote{Vote: vpb}}},
			"328d010a8a010802100122480a206164645f6d6f72655f6578636c616d6174696f6e5f6d61726b735f636f64652d1224080112206164645f6d6f72655f6578636c616d6174696f6e5f6d61726b735f636f64652d32146164645f6d6f72655f6578636c616d6174696f6e38014a220a200000000000000000000000000000000000000000000000000000000000000000"},
		{"HasVote", &tmcons.Message{Sum: &tmcons.Message_HasVote{
			HasVote: &tmcons.HasVote{Height: 1, Round: 1, Type: tmproto.PrevoteType, Index: 1}}},
			"3a080801100118012001"},
		{"HasVote", &tmcons.Message{Sum: &tmcons.Message_HasVote{
			HasVote: &tmcons.HasVote{Height: math.MaxInt64, Round: math.MaxInt32,
				Type: tmproto.PrevoteType, Index: math.MaxInt32}}},
			"3a1808ffffffffffffffff7f10ffffffff07180120ffffffff07"},
		{"VoteSetMaj23", &tmcons.Message{Sum: &tmcons.Message_VoteSetMaj23{
			VoteSetMaj23: &tmcons.VoteSetMaj23{Height: 1, Round: 1, Type: tmproto.PrevoteType, BlockID: pbBi}}},
			"425008011001180122480a206164645f6d6f72655f6578636c616d6174696f6e5f6d61726b735f636f64652d1224080112206164645f6d6f72655f6578636c616d6174696f6e5f6d61726b735f636f64652d"},
		{"VoteSetBits", &tmcons.Message{Sum: &tmcons.Message_VoteSetBits{
			VoteSetBits: &tmcons.VoteSetBits{Height: 1, Round: 1, Type: tmproto.PrevoteType, BlockID: pbBi, Votes: *pbBits}}},
			"4a5708011001180122480a206164645f6d6f72655f6578636c616d6174696f6e5f6d61726b735f636f64652d1224080112206164645f6d6f72655f6578636c616d6174696f6e5f6d61726b735f636f64652d2a050801120100"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			bz, err := proto.Marshal(tc.cMsg)
			require.NoError(t, err)

			require.Equal(t, tc.expBytes, hex.EncodeToString(bz))
		})
	}
}
