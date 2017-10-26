package types_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tendermint/tendermint/types"
	"github.com/tendermint/tmlibs/events"
)

type firer struct {
	w io.Writer
}

var _ events.Fireable = (*firer)(nil)

func (f *firer) FireEvent(evt string, data events.EventData) {
	blob, _ := json.Marshal(data)
	fmt.Fprintf(f.w, "%q:%s\n", evt, blob)
}

func TestEvents(t *testing.T) {
	log := new(bytes.Buffer)
	fr := &firer{w: log}
	sequence := []func(){
		func() { types.FireEventNewBlock(fr, types.EventDataNewBlock{&types.Block{}}) },
		func() { types.FireEventNewBlockHeader(fr, types.EventDataNewBlockHeader{&types.Header{}}) },
		func() { types.FireEventNewRound(fr, types.EventDataRoundState{Height: 10, Round: 1, Step: "uno"}) },
		func() { types.FireEventNewRoundStep(fr, types.EventDataRoundState{Height: 10, Round: 1, Step: "uno"}) },
		func() { types.FireEventTimeoutPropose(fr, types.EventDataRoundState{Height: 8, Round: 2, Step: "foo"}) },
		func() {
			types.FireEventCompleteProposal(fr, types.EventDataRoundState{Height: 2, Round: 4, Step: "val"})
		},
		func() { types.FireEventPolka(fr, types.EventDataRoundState{Height: 2, Step: "tm"}) },
		func() { types.FireEventUnlock(fr, types.EventDataRoundState{Height: 10, Round: 12}) },
		func() { types.FireEventLock(fr, types.EventDataRoundState{Height: 10, Round: 12}) },
		func() { types.FireEventRelock(fr, types.EventDataRoundState{Height: 10, Round: 12}) },
		func() {
			types.FireEventTx(fr, types.EventDataTx{Height: 10, Code: -39876, Log: "events", Error: "invalid rpc"})
		},
		func() {
			types.FireEventTimeoutWait(fr, types.EventDataRoundState{Height: 10, Round: 4, Step: "paused"})
		},
		func() { types.FireEventVote(fr, types.EventDataVote{Vote: &types.Vote{Height: 10}}) },
		func() {
			types.FireEventProposalHeartbeat(fr, types.EventDataProposalHeartbeat{&types.Heartbeat{Height: 25, Sequence: 2}})
		},
	}

	for _, step := range sequence {
		step()
	}

	wantSegments := []string{
		`"NewBlock":{"type":"new_block","data":{"block":{"header":null,"data":null,"last_commit":null}}}`,
		`"NewBlockHeader":{"type":"new_block_header","data":{"header":{"chain_id":"","height":0,"time":"0001-01-01T00:00:00Z","num_txs":0,"last_block_id":{"hash":"","parts":{"total":0,"hash":""}},"last_commit_hash":"","data_hash":"","validators_hash":"","app_hash":""}}}`,
		`"NewRound":{"type":"round_state","data":{"height":10,"round":1,"step":"uno"}}`,
		`"NewRoundStep":{"type":"round_state","data":{"height":10,"round":1,"step":"uno"}}`,
		`"TimeoutPropose":{"type":"round_state","data":{"height":8,"round":2,"step":"foo"}}`,
		`"CompleteProposal":{"type":"round_state","data":{"height":2,"round":4,"step":"val"}}`,
		`"Polka":{"type":"round_state","data":{"height":2,"round":0,"step":"tm"}}`,
		`"Unlock":{"type":"round_state","data":{"height":10,"round":12,"step":""}}`,
		`"Lock":{"type":"round_state","data":{"height":10,"round":12,"step":""}}`,
		`"Relock":{"type":"round_state","data":{"height":10,"round":12,"step":""}}`,
		`"Tx:C81B94933420221A7AC004A90242D8B1D3E5070D":{"type":"tx","data":{"height":10,"tx":null,"data":"","log":"events","code":-39876,"error":"invalid rpc"}}`,
		`"TimeoutWait":{"type":"round_state","data":{"height":10,"round":4,"step":"paused"}}`,
		`"Vote":{"type":"vote","data":{"Vote":{"validator_address":"","validator_index":0,"height":10,"round":0,"type":0,"block_id":{"hash":"","parts":{"total":0,"hash":""}},"signature":null}}}`,
		`"ProposalHeartbeat":{"type":"proposer_heartbeat","data":{"Heartbeat":{"validator_address":"","validator_index":0,"height":25,"round":0,"sequence":2,"signature":null}}}`,
		``, // We need the last to make a new line
	}
	gotLog := log.Bytes()
	wantLog := strings.Join(wantSegments, "\n")
	require.Equal(t, wantLog, string(gotLog), "fire event logs do not match")
}
