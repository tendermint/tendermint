package p2ptest

import (
	gogotypes "github.com/gogo/protobuf/types"
	"github.com/tendermint/tendermint/p2p"
)

// Message is a simple message containing a string-typed Value field.
type Message = gogotypes.StringValue

func NodeInSlice(id p2p.NodeID, ids []p2p.NodeID) bool {
	for _, n := range ids {
		if id == n {
			return true
		}
	}
	return false
}
