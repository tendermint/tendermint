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

func TestNew(t *testing.T) {
	var b map[string]string
	var a StateMachine



}


type B struct {
	Field string
	anotherField []int
	anotherField2 [10]int
}