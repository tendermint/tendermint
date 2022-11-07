package orderbook_test

import (
	"testing"

	"github.com/tendermint/tendermint/abci/example/orderbook"
	"github.com/tendermint/tendermint/abci/types"
)

func TestNew(t *testing.T) {
	StateMachine := New()
	// initialise market
	market := orderbook.NewMarket(testPair)
	// create transaction
	response := S.PrepareProposal(types.RequestPrepareProposal{})

	// check to see if the market collected all of the pairs
	// test to see if the tradeset is valid
	// require.EqualValues(t, )
}
