package blockchain

import (
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blockchain"
	"github.com/tendermint/tendermint/types"
)

const (
	MaxMsgSize = types.MaxBlockSizeBytes +
		bcproto.BlockResponseMessagePrefixSize +
		bcproto.BlockResponseMessageFieldKeySize
)
