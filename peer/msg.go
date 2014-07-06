package peer

import (
	"io"

	. "github.com/tendermint/tendermint/binary"
)

type Message interface {
	Binary
	Type() string
}
