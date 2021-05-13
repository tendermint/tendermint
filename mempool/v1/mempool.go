package v1

import (
	"github.com/tendermint/tendermint/mempool"
)

var _ mempool.Mempool = (*Mempool)(nil)

type TxMempool struct {
}
