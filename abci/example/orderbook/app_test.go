package orderbook_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/example/orderbook"
)

func TestPrepareProposal(t *testing.T) {
	// check to see if the market collected all of the pairs
	market := orderbook.NewMarket(testPair)
	require.EqualValues(t, )
}

