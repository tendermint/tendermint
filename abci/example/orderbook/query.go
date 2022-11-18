package orderbook

// Query the state of an account (returns a concrete copy)
func (sm *StateMachine) Account(id uint64) Account {
	if int(id) >= len(sm.accounts) {
		return Account{}
	}
	return *sm.accounts[id]
}

// Query all the pairs that the orderbook has (returns a concrete copy)
func (sm *StateMachine) Pairs() []Pair {
	pairs := make([]Pair, len(sm.pairs))
	idx := 0
	for _, pair := range sm.pairs {
		pairs[idx] = *pair
		idx++
	}
	return pairs
}

// Query the current orders for a pair (returns concrete copies)
func (sm *StateMachine) Orders(pair *Pair) ([]OrderBid, []OrderAsk) {
	market, ok := sm.markets[pair.String()]
	if !ok {
		return nil, nil
	}
	return market.GetBids(), market.GetAsks()
}

func (sm *StateMachine) Height() int64 { return sm.lastHeight }
