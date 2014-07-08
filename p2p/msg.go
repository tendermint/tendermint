package p2p

import (
	. "github.com/tendermint/tendermint/binary"
)

type Message interface {
	Binary
}
