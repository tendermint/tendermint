package types

import (
	"encoding/json"
	"os"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	"github.com/tendermint/tendermint/internal/jsontypes"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmos "github.com/tendermint/tendermint/libs/os"
)

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

func (nk NodeKey) MarshalJSON() ([]byte, error) {
	pk, err := jsontypes.Marshal(nk.PrivKey)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		ID      NodeID          `json:"id"`
		PrivKey json.RawMessage `json:"priv_key"`
	}{ID: nk.ID, PrivKey: pk})
}

// PubKey returns the peer's PubKey
func (nk NodeKey) PubKey() crypto.PubKey {
	return nk.PrivKey.PubKey()
}

// SaveAs persists the NodeKey to filePath.
func (nk NodeKey) SaveAs(filePath string) error {
	jsonBytes, err := tmjson.Marshal(nk)
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, jsonBytes, 0600)
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
	jsonBytes, err := os.ReadFile(filePath)
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
