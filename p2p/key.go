package p2p

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmos "github.com/tendermint/tendermint/libs/os"
)

// NodeIDByteLength is the length of a crypto.Address. Currently only 20.
// FIXME: support other length addresses?
const NodeIDByteLength = crypto.AddressSize

// NodeID is a hex-encoded crypto.Address.
type NodeID string

// NewNodeID returns a lowercased (normalized) NodeID.
func NewNodeID(nodeID string) (NodeID, error) {
	if _, err := NodeID(nodeID).Bytes(); err != nil {
		return NodeID(""), err
	}

	return NodeID(strings.ToLower(nodeID)), nil
}

// NodeIDFromPubKey returns the noe ID corresponding to the given PubKey. It's
// the hex-encoding of the pubKey.Address().
func NodeIDFromPubKey(pubKey crypto.PubKey) NodeID {
	return NodeID(hex.EncodeToString(pubKey.Address()))
}

// Bytes converts the node ID to it's binary byte representation.
func (id NodeID) Bytes() ([]byte, error) {
	bz, err := hex.DecodeString(string(id))
	if err != nil {
		return nil, fmt.Errorf("invalid node ID encoding: %w", err)
	}
	return bz, nil
}

// Validate validates the NodeID.
func (id NodeID) Validate() error {
	if len(id) == 0 {
		return errors.New("empty node ID")
	}

	bz, err := id.Bytes()
	if err != nil {
		return err
	}

	if len(bz) != NodeIDByteLength {
		return fmt.Errorf("invalid node ID length; got %d, expected %d", len(bz), NodeIDByteLength)
	}

	idStr := string(id)
	if strings.ToLower(idStr) != idStr {
		return fmt.Errorf("invalid node ID; must be lowercased")
	}

	return nil
}

//------------------------------------------------------------------------------
// Persistent peer ID
// TODO: encrypt on disk

// NodeKey is the persistent peer key.
// It contains the nodes private key for authentication.
type NodeKey struct {
	// Canonical ID - hex-encoded pubkey's address (IDByteLength bytes)
	ID NodeID `json:"id"`
	// Private key
	PrivKey crypto.PrivKey `json:"priv_key"`
}

// PubKey returns the peer's PubKey
func (nodeKey NodeKey) PubKey() crypto.PubKey {
	return nodeKey.PrivKey.PubKey()
}

// SaveAs persists the NodeKey to filePath.
func (nodeKey NodeKey) SaveAs(filePath string) error {
	jsonBytes, err := tmjson.Marshal(nodeKey)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(filePath, jsonBytes, 0600)
	if err != nil {
		return err
	}
	return nil
}

// LoadOrGenNodeKey attempts to load the NodeKey from the given filePath. If
// the file does not exist, it generates and saves a new NodeKey.
func LoadOrGenNodeKey(filePath string) (NodeKey, error) {
	if tmos.FileExists(filePath) {
		nodeKey, err := LoadNodeKey(filePath)
		if err != nil {
			return NodeKey{}, err
		}
		return nodeKey, nil
	}

	nodeKey := GenNodeKey()

	if err := nodeKey.SaveAs(filePath); err != nil {
		return NodeKey{}, err
	}

	return nodeKey, nil
}

// GenNodeKey generates a new node key.
func GenNodeKey() NodeKey {
	privKey := ed25519.GenPrivKey()
	return NodeKey{
		ID:      NodeIDFromPubKey(privKey.PubKey()),
		PrivKey: privKey,
	}
}

// LoadNodeKey loads NodeKey located in filePath.
func LoadNodeKey(filePath string) (NodeKey, error) {
	jsonBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return NodeKey{}, err
	}
	nodeKey := NodeKey{}
	err = tmjson.Unmarshal(jsonBytes, &nodeKey)
	if err != nil {
		return NodeKey{}, err
	}
	nodeKey.ID = NodeIDFromPubKey(nodeKey.PubKey())
	return nodeKey, nil
}
