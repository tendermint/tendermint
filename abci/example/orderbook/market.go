package orderbook

import (
	"container/heap"
)

type Market struct {
	pair          Pair     // i.e. EUR/USD (a market is bidirectional)
	buyingOrders  HighestPriceOrderList // i.e. buying EUR for USD
	sellingOrders LowestPriceOrderList // i.e. selling EUR for USD or  buying USD for EUR
}

func (m *Market) AddBid(b MsgBid) error {
	return nil
}

func (m *Market) AddAsk(a MsgAsk) error {
	return nil
}

func (m *Market) Match() (*TradeSet, error) {
	return nil, nil
} 



type LowestPriceOrderList []*Order

func (l LowestPriceOrderList) Less(i, j int) bool {
	return l[i].MinPrice < l[j].MinPrice
}

func (l LowestPriceOrderList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l *LowestPriceOrderList) Push(x any) {
	item := x.(*Order)
	*l = append(*l, item)
}

func (l *LowestPriceOrderList) Pop() any {
	old := *l
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*l = old[0 : n-1]
	return item
}

type HighestPriceOrderList []*Order

func (l HighestPriceOrderList) Less(i, j int) bool {
	return l[i].MinPrice > l[j].MinPrice
}

func (l HighestPriceOrderList) Swap(i, j int) {
	l[i], l[j] = l[j], l[i]
}

func (l *HighestPriceOrderList) Push(x any) {
	item := x.(*Order)
	*l = append(*l, item)
}

func (l *HighestPriceOrderList) Pop() any {
	old := *l
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*l = old[0 : n-1]
	return item
}