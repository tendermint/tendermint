package mempool

import (
	"strconv"
	"sync"
)

const (
	defaultMaxTxNum      int64 = 300
)

var (
	MaxTxNumPerBlock    string
	maxTxNum            int64
	once                sync.Once
)

func string2number(input string, defaultRes int64) int64 {
	if len(input) == 0 {
		return defaultRes
	}

	res, err := strconv.ParseInt(input, 10, 64)
	if err != nil {
		panic(err)
	}
	return res
}

func init() {
	once.Do(func() {
		maxTxNum = string2number(MaxTxNumPerBlock, defaultMaxTxNum)
	})
}

//	query the MaxTxNumPerBlock from app
func (mem *CListMempool) GetMaxTxNumPerBlock() int64 {
	return maxTxNum
}
