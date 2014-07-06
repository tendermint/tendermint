package peer

import (
	. "github.com/tendermint/tendermint/binary"
)

type Message interface {
	Binary
}
