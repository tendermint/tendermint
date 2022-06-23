package log

import (
	"fmt"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

type lazySprintf struct {
	format string
	args   []interface{}
}

// NewLazySprintf defers fmt.Sprintf until the Stringer interface is invoked.
// This is particularly useful for avoiding calling Sprintf when debugging is not
// active.
func NewLazySprintf(format string, args ...interface{}) fmt.Stringer {
	return &lazySprintf{format: format, args: args}
}

func (l *lazySprintf) String() string {
	return fmt.Sprintf(l.format, l.args...)
}

type lazyBlockHash struct {
	block interface{ Hash() tmbytes.HexBytes }
}

// NewLazyBlockHash defers block Hash until the Stringer interface is invoked.
// This is particularly useful for avoiding calling Sprintf when debugging is not
// active.
func NewLazyBlockHash(block interface{ Hash() tmbytes.HexBytes }) fmt.Stringer {
	return &lazyBlockHash{block: block}
}

func (l *lazyBlockHash) String() string {
	return l.block.Hash().String()
}
