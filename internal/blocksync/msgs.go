package blocksync

import (
	"github.com/tendermint/tendermint/pkg/metadata"
	bcproto "github.com/tendermint/tendermint/proto/tendermint/blocksync"
)

const (
	MaxMsgSize = metadata.MaxBlockSizeBytes +
		bcproto.BlockResponseMessagePrefixSize +
		bcproto.BlockResponseMessageFieldKeySize
)
