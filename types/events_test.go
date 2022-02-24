package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Verify that the event data types satisfy their shared interface.
var (
	_ EventData = EventDataBlockSyncStatus{}
	_ EventData = EventDataCompleteProposal{}
	_ EventData = EventDataNewBlock{}
	_ EventData = EventDataNewBlockHeader{}
	_ EventData = EventDataNewEvidence{}
	_ EventData = EventDataNewRound{}
	_ EventData = EventDataRoundState{}
	_ EventData = EventDataStateSyncStatus{}
	_ EventData = EventDataTx{}
	_ EventData = EventDataValidatorSetUpdates{}
	_ EventData = EventDataVote{}
	_ EventData = EventDataString("")
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
		QueryForEvent(EventNewBlockValue).String(),
	)
	assert.Equal(t,
		"tm.event = 'NewEvidence'",
		QueryForEvent(EventNewEvidenceValue).String(),
	)
}
