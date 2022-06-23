package log

import (
	"fmt"

	tmbytes "github.com/tendermint/tendermint/libs/bytes"
)

type lazySprintf struct {
	format string
	args   []interface{}
}

// LazySprintf defers fmt.Sprintf until the Stringer interface is invoked.
// This is particularly useful for avoiding calling Sprintf when debugging is not
// active.
func LazySprintf(format string, args ...interface{}) *lazySprintf { //nolint:revive
	return &lazySprintf{format, args}
}

func (l *lazySprintf) String() string {
	return fmt.Sprintf(l.format, l.args...)
}

func (l *lazySprintf) GoString() string {
	return fmt.Sprintf(l.format, l.args...)
}

type lazyBlockHash struct {
	block hashable
}

type hashable interface {
	Hash() tmbytes.HexBytes
}

// LazyBlockHash defers block Hash until the Stringer interface is invoked.
// This is particularly useful for avoiding calling Sprintf when debugging is not
// active.
func LazyBlockHash(block hashable) *lazyBlockHash { //nolint:revive
	return &lazyBlockHash{block}
}

func (l *lazyBlockHash) String() string {
	return l.block.Hash().String()
}

func (l *lazyBlockHash) GoString() string {
	return l.block.Hash().String()
}
