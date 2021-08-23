package blocksync

import (
	"github.com/tendermint/tendermint/pkg/meta"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blocksync"
)

const (
	MaxMsgSize = meta.MaxBlockSizeBytes +
		bcproto.BlockResponseMessagePrefixSize +
		bcproto.BlockResponseMessageFieldKeySize
)
