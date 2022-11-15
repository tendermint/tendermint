package orderbook_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tendermint/tendermint/abci/example/orderbook"
)

var testPair = &orderbook.Pair{BuyersDenomination: "ATOM", SellersDenomination: "USD"}

func testBid(price, quantity float64) *orderbook.OrderBid {
	return &orderbook.OrderBid{
		MaxPrice:    price,
		MaxQuantity: quantity,
	}
}

func testAsk(price, quantity float64) *orderbook.OrderAsk {
	return &orderbook.OrderAsk{
		AskPrice: price,
		Quantity: quantity,
	}
}

func TestTrackLowestAndHighestPrices(t *testing.T) {
	market := orderbook.NewMarket(testPair)
	require.Zero(t, market.LowestAsk())
	require.Zero(t, market.HighestBid())

	market.AddBid(testBid(100, 10))
	require.EqualValues(t, 100, market.HighestBid())

	market.AddAsk(testAsk(50, 10))
	require.EqualValues(t, 50, market.LowestAsk())

	market.AddAsk(testAsk(30, 10))
	require.EqualValues(t, 30, market.LowestAsk())

	market.AddAsk(testAsk(40, 10))
	require.EqualValues(t, 30, market.LowestAsk())
}

func TestSimpleOrderMatching(t *testing.T) {
	testcases := []struct {
		bid   *orderbook.OrderBid
		ask   *orderbook.OrderAsk
		match bool
	}{
		{
			bid:   testBid(50, 10),
			ask:   testAsk(50, 10),
			match: true,
		},
		{
			bid:   testBid(60, 10),
			ask:   testAsk(50, 10),
			match: true,
		},
		{
			bid:   testBid(50, 10),
			ask:   testAsk(60, 10),
			match: false,
		},
		{
			bid:   testBid(50, 5),
			ask:   testAsk(40, 10),
			match: false,
		},
		{
			bid:   testBid(50, 15),
			ask:   testAsk(40, 10),
			match: true,
		},
	}

	for idx, tc := range testcases {
		market := orderbook.NewMarket(testPair)
		market.AddAsk(tc.ask)
		market.AddBid(tc.bid)
		resp := market.Match()
		if tc.match {
			require.Len(t, resp.MatchedOrders, 1, idx)
		} else {
			require.Nil(t, resp)
		}
	}
}

func TestMultiOrderMatching(t *testing.T) {
	testcases := []struct {
		bids               []*orderbook.OrderBid
		asks               []*orderbook.OrderAsk
		expected           []*orderbook.MatchedOrder
		expectedLowestAsk  float64
		expectedHighestBid float64
	}{
		{
			bids: []*orderbook.OrderBid{
				testBid(50, 20),
				testBid(40, 10),
				testBid(30, 15),
			},
			asks: []*orderbook.OrderAsk{
				testAsk(30, 15),
				testAsk(30, 5),
			},
			expected: []*orderbook.MatchedOrder{
				{
					OrderAsk: testAsk(30, 5),
					OrderBid: testBid(30, 15),
				},
				{
					OrderAsk: testAsk(30, 15),
					OrderBid: testBid(50, 20),
				},
			},
			expectedLowestAsk:  0,
			expectedHighestBid: 40,
		},
		{
			bids: []*orderbook.OrderBid{
				testBid(60, 20),
				testBid(80, 5),
			},
			asks: []*orderbook.OrderAsk{
				testAsk(60, 15),
				testAsk(70, 10),
				testAsk(50, 20),
			},
			expected: []*orderbook.MatchedOrder{
				{
					OrderAsk: testAsk(60, 15),
					OrderBid: testBid(60, 20),
				},
			},
			expectedLowestAsk:  50,
			expectedHighestBid: 80,
		},
		{
			bids: []*orderbook.OrderBid{
				testBid(60, 20),
				testBid(80, 5),
			},
			asks:               []*orderbook.OrderAsk{},
			expected:           []*orderbook.MatchedOrder{},
			expectedLowestAsk:  0,
			expectedHighestBid: 80,
		},
		{
			bids: []*orderbook.OrderBid{},
			asks: []*orderbook.OrderAsk{
				testAsk(70, 10),
				testAsk(50, 20),
			},
			expected:           []*orderbook.MatchedOrder{},
			expectedLowestAsk:  50,
			expectedHighestBid: 0,
		},
	}

	for idx, tc := range testcases {
		market := orderbook.NewMarket(testPair)
		for _, ask := range tc.asks {
			market.AddAsk(ask)
		}
		for _, bid := range tc.bids {
			market.AddBid(bid)
		}
		resp := market.Match()
		if len(tc.expected) == 0 {
			require.Nil(t, resp, idx)
		} else {
			require.Equal(t, tc.expected, resp.MatchedOrders, idx)
		}
		require.EqualValues(t, tc.expectedLowestAsk, market.LowestAsk(), idx)
		require.EqualValues(t, tc.expectedHighestBid, market.HighestBid(), idx)
	}
}
