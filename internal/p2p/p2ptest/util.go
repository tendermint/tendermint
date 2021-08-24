package p2ptest

import (
	gogotypes "github.com/gogo/protobuf/types"
	p2ptypes "github.com/tendermint/tendermint/pkg/p2p"
)

// Message is a simple message containing a string-typed Value field.
type Message = gogotypes.StringValue

func NodeInSlice(id p2ptypes.NodeID, ids []p2ptypes.NodeID) bool {
	for _, n := range ids {
		if id == n {
			return true
		}
	}
	return false
}
