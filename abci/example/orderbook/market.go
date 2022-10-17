package orderbook

import (
	"container/heap"
)

type Market struct {
	pair       Pair      // i.e. EUR/USD (a market is bidirectional)
	askOrders  *AskOrders // i.e. buying EUR for USD
	lowestAsk  float64
	bidOrders  *BidOrders // i.e. selling EUR for USD or  buying USD for EUR
	highestBid float64
}

func NewMarket(p Pair) *Market {
	return &Market{pair: p}
}

func (m *Market) AddBid(b MsgBid) {
	heap.Push(m.bidOrders, b)
	if b.BidOrder.MaxPrice > m.highestBid {
		m.highestBid = b.BidOrder.MaxPrice
	}
}

func (m *Market) AddAsk(a MsgAsk) {
	heap.Push(m.askOrders, a)
	if a.AskOrder.AskPrice < m.lowestAsk {
		m.lowestAsk = a.AskOrder.AskPrice
	}
}

// Match takes the set of bids and asks and matches them together.
// A bid matches an ask when the MaxPrice is greater than the AskPrice
// and the MaxQuantity is greater than the quantity. 
func (m *Market) Match() *TradeSet {
	if m.highestBid < m.lowestAsk {
		// no orders match, we return early.
		return nil
	}

	t := &TradeSet{Pair: &m.pair}
	bids := make([]*OrderBid, 0)
	asks := make([]*OrderAsk, 0)

	// get all the bids that are greater than the lowest ask. In order from heighest bid to lowest bid
	for bid := heap.Pop(m.bidOrders).(*OrderBid); bid.MaxPrice >= m.lowestAsk; bid = heap.Pop(m.bidOrders).(*OrderBid) {
		bids = append(bids, bid)
	}

	// get all the asks that are lower than the highest bid in the bids set. Ordered from lowest to highest ask
	for ask := heap.Pop(m.askOrders).(*OrderAsk); ask.AskPrice <= bids[0].MaxPrice; ask = heap.Pop(m.askOrders).(*OrderAsk) {
		asks = append(asks, ask)
	}

	// this is to keep track of the index of the bids that have been matched
	reserved := make(map[int]struct{})

	// start from the highest ask and the highest bid and for each ask loop downwards through the slice of
	// bids until one is matched
OUTER_LOOP:
	for i := len(asks) - 1; i >= 0; i-- {
		ask := asks[i]

		// start with the highest bid and increment down since we're more likely to find a match
		for j := 0 ; j < len(bids); j++ {
			bid := bids[j]
			if bid.MaxPrice >= ask.AskPrice {
				if bid.MaxQuantity >= ask.Quantity {
					// yay! we have a match
					t.AddFilledOrder(ask, bid)

					// reserve the bid so we don't rematch it with another ask
					reserved[j] = struct{}{}
					continue OUTER_LOOP
				}
			} else {
				// once we've dropped below the ask price there are no more possible bids and so we break
				break
			}
		}

		// as we go from highest to lowest, asks that aren't matched become the new lowest ask price
		m.lowestAsk = ask.AskPrice

		// no match found, add the ask order back into the heap
		heap.Push(m.askOrders, ask)
	}

	// add back the unmatched bids to the heap so they can be matched again in a later round.
	// We also neeed to recalculate the new highest bid. First we tackle an edge case whereby all
	// selected bids were matched. In this case we grab the next highest and set that as the new
	// highest bid
	m.highestBid = 0
	if len(reserved) == len(bids) {
		newHighestBid := heap.Pop(m.bidOrders).(*OrderBid)
		m.highestBid = newHighestBid.MaxPrice
		heap.Push(m.bidOrders, newHighestBid)
	}
	for j := 0; j < len(bids); j++ {
		if _, ok := reserved[j]; !ok {
			if bids[j].MaxPrice > m.highestBid {
				m.highestBid = bids[j].MaxPrice
			}
			heap.Push(m.bidOrders, bids[j])
		}
	}

	return t
}

func (m Market) LowestAsk() float64 {
	return m.lowestAsk
}

func (m Market) HighestBid() float64 {
	return m.highestBid
}

// Heap ordered by lowest price
type AskOrders []*OrderAsk

var _ heap.Interface = (*AskOrders)(nil)

func (a AskOrders) Len() int { return len(a) }

func (a AskOrders) Less(i, j int) bool {
	return a[i].AskPrice < a[j].AskPrice
}

func (a AskOrders) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a *AskOrders) Push(x any) {
	item := x.(*OrderAsk)
	*a = append(*a, item)
}

func (a *AskOrders) Pop() any {
	old := *a
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*a = old[0 : n-1]
	return item
}

// Heap ordered by highest price
type BidOrders []*OrderBid

var _ heap.Interface = (*BidOrders)(nil)

func (b BidOrders) Len() int { return len(b) }

func (b BidOrders) Less(i, j int) bool {
	return b[i].MaxPrice > b[j].MaxPrice
}

func (b BidOrders) Swap(i, j int) {
	b[i], b[j] = b[j], b[i]
}

func (b *BidOrders) Push(x any) {
	item := x.(*OrderBid)
	*b = append(*b, item)
}

func (b *BidOrders) Pop() any {
	old := *b
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*b = old[0 : n-1]
	return item
}

func (t *TradeSet) AddFilledOrder(ask *OrderAsk, bid *OrderBid) {
	t.MatchedOrders = append(t.MatchedOrders, &MatchedOrder{
		OrderAsk: ask,
		OrderBid: bid,
	})
}
