package p2ptest

import (
	gogotypes "github.com/gogo/protobuf/types"

	"github.com/tendermint/tendermint/types"
)

// Message is a simple message containing a string-typed Value field.
type Message = gogotypes.StringValue

func NodeInSlice(id types.NodeID, ids []types.NodeID) bool {
	for _, n := range ids {
		if id == n {
			return true
		}
	}
	return false
}
