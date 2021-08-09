package events

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tendermint/tendermint/pkg/mempool"
)

func TestQueryTxFor(t *testing.T) {
	tx := mempool.Tx("foo")
	assert.Equal(t,
		fmt.Sprintf("tm.event='Tx' AND tx.hash='%X'", tx.Hash()),
		EventQueryTxFor(tx).String(),
	)
}

func TestQueryForEvent(t *testing.T) {
	assert.Equal(t,
		"tm.event='NewBlock'",
		QueryForEvent(EventNewBlockValue).String(),
	)
	assert.Equal(t,
		"tm.event='NewEvidence'",
		QueryForEvent(EventNewEvidenceValue).String(),
	)
}
