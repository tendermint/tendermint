package consensus

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	tmtypes "github.com/tendermint/tendermint/types"
)

func TestWalIter_Next(t *testing.T) {
	testCases := []struct {
		input []interface{}
		want  []Message
	}{
		{},
		{
			input: []interface{}{
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1000, Round: 0}},
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1001, Round: 0}},
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1002, Round: 0}},
			},
			want: []Message{
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1000, Round: 0}},
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1001, Round: 0}},
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1002, Round: 0}},
			},
		},
		{
			input: []interface{}{
				&VoteMessage{Vote: &tmtypes.Vote{Height: 0, Round: 0}},
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1000, Round: 0}},
				&BlockPartMessage{Height: 1000, Round: 0},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1000, Round: 0}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1000, Round: 0}},
			},
			want: []Message{
				&VoteMessage{Vote: &tmtypes.Vote{Height: 0, Round: 0}},
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1000, Round: 0}},
				&BlockPartMessage{Height: 1000, Round: 0},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1000, Round: 0}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1000, Round: 0}},
			},
		},
		{
			input: []interface{}{
				&VoteMessage{Vote: &tmtypes.Vote{Height: 0, Round: 0}},
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1000, Round: 0}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1000, Round: 0}},
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1001, Round: 0}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1000, Round: 0}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1000, Round: 1}},
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1001, Round: 1}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1001, Round: 0}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1001, Round: 1}},
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1001, Round: 2}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1001, Round: 0}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1001, Round: 1}},
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1001, Round: 3}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1001, Round: 0}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1001, Round: 1}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1001, Round: 2}},
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1002, Round: 0}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1002, Round: 0}},
			},
			want: []Message{
				&VoteMessage{Vote: &tmtypes.Vote{Height: 0, Round: 0}},
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1000, Round: 0}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1000, Round: 0}},
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1001, Round: 3}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1001, Round: 0}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1001, Round: 1}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1001, Round: 2}},
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1002, Round: 0}},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1002, Round: 0}},
			},
		},
		{
			input: generateMessageSequence(1000, 1002, 1000),
			want: []Message{
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1000, Round: 1000}},
				&BlockPartMessage{Height: 1000, Round: 1000},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1000, Round: 1000}},

				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1001, Round: 1000}},
				&BlockPartMessage{Height: 1001, Round: 1000},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1001, Round: 1000}},

				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: 1002, Round: 1000}},
				&BlockPartMessage{Height: 1002, Round: 1000},
				&VoteMessage{Vote: &tmtypes.Vote{Height: 1002, Round: 1000}},
			},
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test-case #%d", i), func(t *testing.T) {
			it := newWalIter(&mockWALReader{ret: tc.input})
			cnt := 0
			for it.Next() {
				msg := it.Value()
				mi := msg.Msg.(msgInfo)
				require.Equal(t, tc.want[cnt], mi.Msg)
				cnt++
			}
			require.NoError(t, it.Err())
			require.Equal(t, len(tc.want), cnt)
		})
	}
}

func TestWalIter_Err(t *testing.T) {
	testCases := []struct {
		err  error
		want string
	}{
		{
			err: io.EOF,
		},
		{
			err:  errors.New("reader error"),
			want: "reader error",
		},
	}
	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test-case #%d", i), func(t *testing.T) {
			it := newWalIter(&mockWALReader{ret: []interface{}{
				tc.err,
			}})
			require.False(t, it.Next())
			requireError(t, tc.want, it.Err())
		})
	}
}

func requireError(t *testing.T, want string, err error) {
	if err != nil {
		require.True(t, strings.Contains(err.Error(), want))
		return
	}
	require.Equal(t, "", want, "error must not be nil")
}

type mockWALReader struct {
	ret []interface{}
	pos int
}

func (w *mockWALReader) Decode() (*TimedWALMessage, error) {
	l := len(w.ret)
	if l == 0 || w.pos >= l {
		return nil, io.EOF
	}
	r := w.ret[w.pos]
	var (
		twm *TimedWALMessage
		err error
	)
	switch val := r.(type) {
	case Message:
		twm = &TimedWALMessage{
			Msg: msgInfo{Msg: val},
		}
	case error:
		err = val
	default:
		err = fmt.Errorf("unsupported a type, allowed [Message, error]; given %T", r)
	}
	w.pos++
	return twm, err
}

func generateMessageSequence(heightStart, heightEnd int64, roundEnd int32) []interface{} {
	var msgs []interface{}
	for h := heightStart; h <= heightEnd; h++ {
		for r := int32(0); r <= roundEnd; r++ {
			msgs = append(
				msgs,
				&ProposalMessage{Proposal: &tmtypes.Proposal{Height: h, Round: r}},
				&BlockPartMessage{Height: h, Round: r},
				&VoteMessage{Vote: &tmtypes.Vote{Height: h, Round: r}},
			)
		}
	}
	return msgs
}
