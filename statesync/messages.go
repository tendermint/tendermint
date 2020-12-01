package statesync

import (
	"github.com/tendermint/tendermint/p2p"
	ssproto "github.com/tendermint/tendermint/proto/tendermint/statesync"
)

const (
	// snapshotMsgSize is the maximum size of a snapshotResponseMessage
	snapshotMsgSize = int(4e6)
	// chunkMsgSize is the maximum size of a chunkResponseMessage
	chunkMsgSize = int(16e6)
)

// assert Wrapper interface implementation of the state sync proto message type.
var _ p2p.Wrapper = (*ssproto.Message)(nil)
