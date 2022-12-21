package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Verify that the event data types satisfy their shared interface.
// TODO: add EventDataBlockSyncStatus and EventDataStateSyncStatus
// when backport #6700 and #6755.
var (
	_ TMEventData = EventDataCompleteProposal{}
	_ TMEventData = EventDataNewBlock{}
	_ TMEventData = EventDataNewBlockHeader{}
	_ TMEventData = EventDataNewEvidence{}
	_ TMEventData = EventDataNewRound{}
	_ TMEventData = EventDataRoundState{}
	_ TMEventData = EventDataTx{}
	_ TMEventData = EventDataValidatorSetUpdates{}
	_ TMEventData = EventDataVote{}
	_ TMEventData = EventDataString("")
)

func TestQueryTxFor(t *testing.T) {
	tx := Tx("foo")
	assert.Equal(t,
		fmt.Sprintf("tm.event = 'Tx' AND tx.hash = '%X'", tx.Hash()),
		EventQueryTxFor(tx).String(),
	)
}

func TestQueryForEvent(t *testing.T) {
	assert.Equal(t,
		"tm.event = 'NewBlock'",
		QueryForEvent(EventNewBlock).String(),
	)
	assert.Equal(t,
		"tm.event = 'NewEvidence'",
		QueryForEvent(EventNewEvidence).String(),
	)
}
