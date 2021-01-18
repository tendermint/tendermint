package blockchain

import (
	"fmt"

	bc "github.com/tendermint/tendermint/blockchain"
)

func Fuzz(data []byte) int {
	msg, err := bc.DecodeMsg(data)
	if err != nil {
		if msg != nil {
			panic(fmt.Sprintf("msg %v != nil on error", msg))
		}
		return 0
	}

	_, err = bc.EncodeMsg(msg)
	if err != nil {
		panic(err)
	}

	return 1
}
