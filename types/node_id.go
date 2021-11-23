package types

import (
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/tendermint/tendermint/crypto"
)

// NodeIDByteLength is the length of a crypto.Address. Currently only 20.
// FIXME: support other length addresses?
const NodeIDByteLength = crypto.AddressSize

// reNodeID is a regexp for valid node IDs.
var reNodeID = regexp.MustCompile(`^[0-9a-f]{40}$`)

// NodeID is a hex-encoded crypto.Address. It must be lowercased
// (for uniqueness) and of length 2*NodeIDByteLength.
type NodeID string

// NewNodeID returns a lowercased (normalized) NodeID, or errors if the
// node ID is invalid.
func NewNodeID(nodeID string) (NodeID, error) {
	n := NodeID(strings.ToLower(nodeID))
	return n, n.Validate()
}

// IDAddressString returns id@hostPort. It strips the leading
// protocol from protocolHostPort if it exists.
func (id NodeID) AddressString(protocolHostPort string) string {
	return fmt.Sprintf("%s@%s", id, removeProtocolIfDefined(protocolHostPort))
}

// NodeIDFromPubKey creates a node ID from a given PubKey address.
func NodeIDFromPubKey(pubKey crypto.PubKey) NodeID {
	return NodeID(hex.EncodeToString(pubKey.Address()))
}

// Bytes converts the node ID to its binary byte representation.
func (id NodeID) Bytes() ([]byte, error) {
	bz, err := hex.DecodeString(string(id))
	if err != nil {
		return nil, fmt.Errorf("invalid node ID encoding: %w", err)
	}
	return bz, nil
}

// Validate validates the NodeID.
func (id NodeID) Validate() error {
	switch {
	case len(id) == 0:
		return errors.New("empty node ID")

	case len(id) != 2*NodeIDByteLength:
		return fmt.Errorf("invalid node ID length %d, expected %d", len(id), 2*NodeIDByteLength)

	case !reNodeID.MatchString(string(id)):
		return fmt.Errorf("node ID can only contain lowercased hex digits")

	default:
		return nil
	}
}
