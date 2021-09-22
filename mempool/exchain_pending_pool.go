package mempool

import (
	"sync"

	"github.com/tendermint/tendermint/types"
)

type PendingPool struct {
	maxSize         int
	addressTxsMap   map[string]map[uint64]*PendingTx
	txsMap          map[string]*PendingTx
	mtx             sync.RWMutex
	period          int
	reserveBlocks   int
	periodCounter   map[string]int // address with period count
	maxTxPerAddress int
}

func newPendingPool(maxSize int, period int, reserveBlocks int, maxTxPerAddress int) *PendingPool {
	return &PendingPool{
		maxSize:         maxSize,
		addressTxsMap:   make(map[string]map[uint64]*PendingTx),
		txsMap:          make(map[string]*PendingTx),
		period:          period,
		reserveBlocks:   reserveBlocks,
		periodCounter:   make(map[string]int),
		maxTxPerAddress: maxTxPerAddress,
	}
}

func (p *PendingPool) Size() int {
	p.mtx.RLock()
	defer p.mtx.RUnlock()
	return len(p.txsMap)
}

func (p *PendingPool) txCount(address string) int {
	p.mtx.RLock()
	defer p.mtx.RUnlock()
	if _, ok := p.addressTxsMap[address]; !ok {
		return 0
	}
	return len(p.addressTxsMap[address])
}

func (p *PendingPool) getTx(address string, nonce uint64) *PendingTx {
	p.mtx.RLock()
	defer p.mtx.RUnlock()
	if _, ok := p.addressTxsMap[address]; ok {
		return p.addressTxsMap[address][nonce]
	}
	return nil
}

func (p *PendingPool) hasTx(tx types.Tx) bool {
	p.mtx.RLock()
	defer p.mtx.RUnlock()
	_, exist := p.txsMap[txID(tx)]
	return exist
}

func (p *PendingPool) addTx(pendingTx *PendingTx) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	if _, ok := p.addressTxsMap[pendingTx.exTxInfo.Sender]; !ok {
		p.addressTxsMap[pendingTx.exTxInfo.Sender] = make(map[uint64]*PendingTx)
	}
	p.addressTxsMap[pendingTx.exTxInfo.Sender][pendingTx.exTxInfo.Nonce] = pendingTx
	p.txsMap[txID(pendingTx.mempoolTx.tx)] = pendingTx
}

func (p *PendingPool) removeTx(address string, nonce uint64) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	if _, ok := p.addressTxsMap[address]; ok {
		if pendingTx, ok := p.addressTxsMap[address][nonce]; ok {
			delete(p.addressTxsMap[address], nonce)
			delete(p.txsMap, txID(pendingTx.mempoolTx.tx))
		}
		if len(p.addressTxsMap[address]) == 0 {
			delete(p.addressTxsMap, address)
			delete(p.periodCounter, address)
		}
		// update period counter
		if count, ok := p.periodCounter[address]; ok && count > 0 {
			p.periodCounter[address] = count - 1
		}

	}

}

func (p *PendingPool) removeTxByHash(txHash string) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	if pendingTx, ok := p.txsMap[txHash]; ok {
		delete(p.txsMap, txHash)
		if _, ok := p.addressTxsMap[pendingTx.exTxInfo.Sender]; ok {
			delete(p.addressTxsMap[pendingTx.exTxInfo.Sender], pendingTx.exTxInfo.Nonce)
			if len(p.addressTxsMap[pendingTx.exTxInfo.Sender]) == 0 {
				delete(p.addressTxsMap, pendingTx.exTxInfo.Sender)
				delete(p.periodCounter, pendingTx.exTxInfo.Sender)
			}
			// update period counter
			if count, ok := p.periodCounter[pendingTx.exTxInfo.Sender]; ok && count > 0 {
				p.periodCounter[pendingTx.exTxInfo.Sender] = count - 1
			}
		}
	}
}

func (p *PendingPool) handlePendingTx(addressNonce map[string]uint64) map[string]uint64 {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	addrMap := make(map[string]uint64)
	for addr, accountNonce := range addressNonce {
		if txsMap, ok := p.addressTxsMap[addr]; ok {
			for nonce, pendingTx := range txsMap {
				// remove invalid pending tx
				if nonce <= accountNonce {
					delete(p.addressTxsMap[addr], nonce)
					delete(p.txsMap, txID(pendingTx.mempoolTx.tx))
				} else if nonce == accountNonce+1 {
					addrMap[addr] = nonce
				}
			}
			if len(p.addressTxsMap[addr]) == 0 {
				delete(p.addressTxsMap, addr)
			}
		}
	}
	return addrMap
}

func (p *PendingPool) handlePeriodCounter() {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	for addr, txMap := range p.addressTxsMap {
		count := p.periodCounter[addr]
		if count >= p.reserveBlocks {
			delete(p.addressTxsMap, addr)
			for _, pendingTx := range txMap {
				delete(p.txsMap, txID(pendingTx.mempoolTx.tx))
			}
			delete(p.periodCounter, addr)
		} else {
			p.periodCounter[addr] = count + 1
		}
	}
}

func (p *PendingPool) validate(address string, tx types.Tx) error {
	// tx already in pending pool
	if p.hasTx(tx) {
		return ErrTxAlreadyInPendingPool{
			txHash: txID(tx),
		}
	}

	poolSize := p.Size()
	if poolSize >= p.maxSize {
		return ErrPendingPoolIsFull{
			size:    poolSize,
			maxSize: p.maxSize,
		}
	}
	txCount := p.txCount(address)
	if txCount >= p.maxTxPerAddress {
		return ErrPendingPoolAddressLimit{
			address: address,
			size:    txCount,
			maxSize: p.maxTxPerAddress,
		}
	}
	return nil
}

type PendingTx struct {
	mempoolTx *mempoolTx
	exTxInfo  ExTxInfo
}

type AccountRetriever interface {
	GetAccountNonce(address string) uint64
}
