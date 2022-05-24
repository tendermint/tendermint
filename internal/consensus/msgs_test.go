package consensus

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/merkle"
	cstypes "github.com/tendermint/tendermint/internal/consensus/types"
	"github.com/tendermint/tendermint/internal/test/factory"
	"github.com/tendermint/tendermint/libs/bits"
	"github.com/tendermint/tendermint/libs/bytes"
	tmrand "github.com/tendermint/tendermint/libs/rand"
	tmcons "github.com/tendermint/tendermint/proto/tendermint/consensus"
	tmproto "github.com/tendermint/tendermint/proto/tendermint/types"
	"github.com/tendermint/tendermint/types"
)

func TestMsgToProto(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
		Type:      tmproto.ProposalType,
		Height:    1,
		Round:     1,
		POLRound:  1,
		BlockID:   bi,
		Timestamp: time.Now(),
		Signature: tmrand.Bytes(20),
	}
	pbProposal := proposal.ToProto()

	pv := types.NewMockPV()
	vote, err := factory.MakeVote(ctx, pv, factory.DefaultTestChainID,
		0, 1, 0, 2, bi, time.Now())
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
			PeerID: types.NodeID("string"),
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
		Type:      tmproto.ProposalType,
		Height:    1,
		Round:     1,
		POLRound:  1,
		BlockID:   bi,
		Timestamp: date,
		Signature: []byte("add_more_exclamation"),
	}
	pbProposal := proposal.ToProto()

	v := &types.Vote{
		ValidatorAddress: []byte("add_more_exclamation"),
		ValidatorIndex:   1,
		Height:           1,
		Round:            0,
		Timestamp:        date,
		Type:             tmproto.PrecommitType,
		BlockID:          bi,
		Extension:        []byte("extension"),
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
			"1a720a7008201001180120012a480a206164645f6d6f72655f6578636c616d6174696f6e5f6d61726b735f636f64652d1224080112206164645f6d6f72655f6578636c616d6174696f6e5f6d61726b735f636f64652d320608c0b89fdc053a146164645f6d6f72655f6578636c616d6174696f6e"},
		{"ProposalPol", &tmcons.Message{Sum: &tmcons.Message_ProposalPol{
			ProposalPol: &tmcons.ProposalPOL{Height: 1, ProposalPolRound: 1}}},
			"2206080110011a00"},
		{"BlockPart", &tmcons.Message{Sum: &tmcons.Message_BlockPart{
			BlockPart: &tmcons.BlockPart{Height: 1, Round: 1, Part: *pbParts}}},
			"2a36080110011a3008011204746573741a26080110011a206164645f6d6f72655f6578636c616d6174696f6e5f6d61726b735f636f64652d"},
		{"Vote", &tmcons.Message{Sum: &tmcons.Message_Vote{
			Vote: &tmcons.Vote{Vote: vpb}}},
			"327b0a790802100122480a206164645f6d6f72655f6578636c616d6174696f6e5f6d61726b735f636f64652d1224080112206164645f6d6f72655f6578636c616d6174696f6e5f6d61726b735f636f64652d2a0608c0b89fdc0532146164645f6d6f72655f6578636c616d6174696f6e38014a09657874656e73696f6e"},
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

func TestVoteSetMaj23MessageValidateBasic(t *testing.T) {
	const (
		validSignedMsgType   tmproto.SignedMsgType = 0x01
		invalidSignedMsgType tmproto.SignedMsgType = 0x03
	)

	validBlockID := types.BlockID{}
	invalidBlockID := types.BlockID{
		Hash: bytes.HexBytes{},
		PartSetHeader: types.PartSetHeader{
			Total: 1,
			Hash:  []byte{0},
		},
	}

	testCases := []struct { // nolint: maligned
		expectErr      bool
		messageRound   int32
		messageHeight  int64
		testName       string
		messageType    tmproto.SignedMsgType
		messageBlockID types.BlockID
	}{
		{false, 0, 0, "Valid Message", validSignedMsgType, validBlockID},
		{true, -1, 0, "Invalid Message", validSignedMsgType, validBlockID},
		{true, 0, -1, "Invalid Message", validSignedMsgType, validBlockID},
		{true, 0, 0, "Invalid Message", invalidSignedMsgType, validBlockID},
		{true, 0, 0, "Invalid Message", validSignedMsgType, invalidBlockID},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			message := VoteSetMaj23Message{
				Height:  tc.messageHeight,
				Round:   tc.messageRound,
				Type:    tc.messageType,
				BlockID: tc.messageBlockID,
			}

			assert.Equal(t, tc.expectErr, message.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}

func TestVoteSetBitsMessageValidateBasic(t *testing.T) {
	testCases := []struct {
		malleateFn func(*VoteSetBitsMessage)
		expErr     string
	}{
		{func(msg *VoteSetBitsMessage) {}, ""},
		{func(msg *VoteSetBitsMessage) { msg.Height = -1 }, "negative Height"},
		{func(msg *VoteSetBitsMessage) { msg.Type = 0x03 }, "invalid Type"},
		{func(msg *VoteSetBitsMessage) {
			msg.BlockID = types.BlockID{
				Hash: bytes.HexBytes{},
				PartSetHeader: types.PartSetHeader{
					Total: 1,
					Hash:  []byte{0},
				},
			}
		}, "wrong BlockID: wrong PartSetHeader: wrong Hash:"},
		{func(msg *VoteSetBitsMessage) { msg.Votes = bits.NewBitArray(types.MaxVotesCount + 1) },
			"votes bit array is too big: 10001, max: 10000"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			msg := &VoteSetBitsMessage{
				Height:  1,
				Round:   0,
				Type:    0x01,
				Votes:   bits.NewBitArray(1),
				BlockID: types.BlockID{},
			}

			tc.malleateFn(msg)
			err := msg.ValidateBasic()
			if tc.expErr != "" && assert.Error(t, err) {
				assert.Contains(t, err.Error(), tc.expErr)
			}
		})
	}
}

func TestNewRoundStepMessageValidateBasic(t *testing.T) {
	testCases := []struct { // nolint: maligned
		expectErr              bool
		messageRound           int32
		messageLastCommitRound int32
		messageHeight          int64
		testName               string
		messageStep            cstypes.RoundStepType
	}{
		{false, 0, 0, 0, "Valid Message", cstypes.RoundStepNewHeight},
		{true, -1, 0, 0, "Negative round", cstypes.RoundStepNewHeight},
		{true, 0, 0, -1, "Negative height", cstypes.RoundStepNewHeight},
		{true, 0, 0, 0, "Invalid Step", cstypes.RoundStepCommit + 1},
		// The following cases will be handled by ValidateHeight
		{false, 0, 0, 1, "H == 1 but LCR != -1 ", cstypes.RoundStepNewHeight},
		{false, 0, -1, 2, "H > 1 but LCR < 0", cstypes.RoundStepNewHeight},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			message := NewRoundStepMessage{
				Height:          tc.messageHeight,
				Round:           tc.messageRound,
				Step:            tc.messageStep,
				LastCommitRound: tc.messageLastCommitRound,
			}

			err := message.ValidateBasic()
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewRoundStepMessageValidateHeight(t *testing.T) {
	initialHeight := int64(10)
	testCases := []struct { // nolint: maligned
		expectErr              bool
		messageLastCommitRound int32
		messageHeight          int64
		testName               string
	}{
		{false, 0, 11, "Valid Message"},
		{true, 0, -1, "Negative height"},
		{true, 0, 0, "Zero height"},
		{true, 0, 10, "Initial height but LCR != -1 "},
		{true, -1, 11, "Normal height but LCR < 0"},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			message := NewRoundStepMessage{
				Height:          tc.messageHeight,
				Round:           0,
				Step:            cstypes.RoundStepNewHeight,
				LastCommitRound: tc.messageLastCommitRound,
			}

			err := message.ValidateHeight(initialHeight)
			if tc.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNewValidBlockMessageValidateBasic(t *testing.T) {
	testCases := []struct {
		malleateFn func(*NewValidBlockMessage)
		expErr     string
	}{
		{func(msg *NewValidBlockMessage) {}, ""},
		{func(msg *NewValidBlockMessage) { msg.Height = -1 }, "negative Height"},
		{func(msg *NewValidBlockMessage) { msg.Round = -1 }, "negative Round"},
		{
			func(msg *NewValidBlockMessage) { msg.BlockPartSetHeader.Total = 2 },
			"blockParts bit array size 1 not equal to BlockPartSetHeader.Total 2",
		},
		{
			func(msg *NewValidBlockMessage) {
				msg.BlockPartSetHeader.Total = 0
				msg.BlockParts = bits.NewBitArray(0)
			},
			"empty blockParts",
		},
		{
			func(msg *NewValidBlockMessage) { msg.BlockParts = bits.NewBitArray(int(types.MaxBlockPartsCount) + 1) },
			"blockParts bit array size 1602 not equal to BlockPartSetHeader.Total 1",
		},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			msg := &NewValidBlockMessage{
				Height: 1,
				Round:  0,
				BlockPartSetHeader: types.PartSetHeader{
					Total: 1,
				},
				BlockParts: bits.NewBitArray(1),
			}

			tc.malleateFn(msg)
			err := msg.ValidateBasic()
			if tc.expErr != "" && assert.Error(t, err) {
				assert.Contains(t, err.Error(), tc.expErr)
			}
		})
	}
}

func TestProposalPOLMessageValidateBasic(t *testing.T) {
	testCases := []struct {
		malleateFn func(*ProposalPOLMessage)
		expErr     string
	}{
		{func(msg *ProposalPOLMessage) {}, ""},
		{func(msg *ProposalPOLMessage) { msg.Height = -1 }, "negative Height"},
		{func(msg *ProposalPOLMessage) { msg.ProposalPOLRound = -1 }, "negative ProposalPOLRound"},
		{func(msg *ProposalPOLMessage) { msg.ProposalPOL = bits.NewBitArray(0) }, "empty ProposalPOL bit array"},
		{func(msg *ProposalPOLMessage) { msg.ProposalPOL = bits.NewBitArray(types.MaxVotesCount + 1) },
			"proposalPOL bit array is too big: 10001, max: 10000"},
	}

	for i, tc := range testCases {
		tc := tc
		t.Run(fmt.Sprintf("#%d", i), func(t *testing.T) {
			msg := &ProposalPOLMessage{
				Height:           1,
				ProposalPOLRound: 1,
				ProposalPOL:      bits.NewBitArray(1),
			}

			tc.malleateFn(msg)
			err := msg.ValidateBasic()
			if tc.expErr != "" && assert.Error(t, err) {
				assert.Contains(t, err.Error(), tc.expErr)
			}
		})
	}
}

func TestBlockPartMessageValidateBasic(t *testing.T) {
	testPart := new(types.Part)
	testPart.Proof.LeafHash = crypto.Checksum([]byte("leaf"))
	testCases := []struct {
		testName      string
		messageHeight int64
		messageRound  int32
		messagePart   *types.Part
		expectErr     bool
	}{
		{"Valid Message", 0, 0, testPart, false},
		{"Invalid Message", -1, 0, testPart, true},
		{"Invalid Message", 0, -1, testPart, true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			message := BlockPartMessage{
				Height: tc.messageHeight,
				Round:  tc.messageRound,
				Part:   tc.messagePart,
			}

			assert.Equal(t, tc.expectErr, message.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}

	message := BlockPartMessage{Height: 0, Round: 0, Part: new(types.Part)}
	message.Part.Index = 1

	assert.Equal(t, true, message.ValidateBasic() != nil, "Validate Basic had an unexpected result")
}

func TestHasVoteMessageValidateBasic(t *testing.T) {
	const (
		validSignedMsgType   tmproto.SignedMsgType = 0x01
		invalidSignedMsgType tmproto.SignedMsgType = 0x03
	)

	testCases := []struct { // nolint: maligned
		expectErr     bool
		messageRound  int32
		messageIndex  int32
		messageHeight int64
		testName      string
		messageType   tmproto.SignedMsgType
	}{
		{false, 0, 0, 0, "Valid Message", validSignedMsgType},
		{true, -1, 0, 0, "Invalid Message", validSignedMsgType},
		{true, 0, -1, 0, "Invalid Message", validSignedMsgType},
		{true, 0, 0, 0, "Invalid Message", invalidSignedMsgType},
		{true, 0, 0, -1, "Invalid Message", validSignedMsgType},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			message := HasVoteMessage{
				Height: tc.messageHeight,
				Round:  tc.messageRound,
				Type:   tc.messageType,
				Index:  tc.messageIndex,
			}

			assert.Equal(t, tc.expectErr, message.ValidateBasic() != nil, "Validate Basic had an unexpected result")
		})
	}
}
