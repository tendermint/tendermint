package orderbook_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/example/orderbook"
)

var testPair = orderbook.Pair{BuyersDenomination: "ATOM", SellersDenomination: "USD"}

func TestTrackLowestAndHighestPrices(t *testing.T) {
	market := orderbook.NewMarket(testPair)
	require.Zero(t, market.LowestAsk())
	require.Zero(t, market.HighestBid())

	market.AddBid(orderbook.OrderBid{MaxPrice: 100})
	require.Equal(t, 100, market.HighestBid())

	market.AddAsk(orderbook.OrderAsk{AskPrice: 50})
	require.Equal(t, 50, market.LowestAsk())

	market.AddAsk(orderbook.OrderAsk{AskPrice: 30})
	require.Equal(t, 30, market.LowestAsk())

	market.AddAsk(orderbook.OrderAsk{AskPrice: 40})
	require.Equal(t, 30, market.LowestAsk())
}

func TestSimpleOrderMatching(t *testing.T) {
	testcases := []struct {
		bid orderbook.OrderBid
		ask orderbook.OrderAsk
		match bool
	}{
		{
			bid: orderbook.OrderBid{
				MaxPrice: 50,
				MaxQuantity: 10,
			},
			ask: orderbook.OrderAsk{
				AskPrice: 50,
				Quantity: 10,
			},
			match: true,
		},
	}

	for idx, tc := range testcases {
		market := orderbook.NewMarket(testPair)
		market.AddAsk(tc.ask)
		market.AddBid(tc.bid)
		resp := market.Match()
		require.Equal(t, tc.match, len(resp.MatchedOrders) == 1, idx)
	}
}