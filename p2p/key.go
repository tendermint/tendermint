package p2p

import (
	"encoding/hex"
	"io/ioutil"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/crypto/ed25519"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmos "github.com/tendermint/tendermint/libs/os"
)

// ID is a hex-encoded crypto.Address
type ID string

// IDByteLength is the length of a crypto.Address. Currently only 20.
// TODO: support other length addresses ?
const IDByteLength = crypto.AddressSize

//------------------------------------------------------------------------------
// Persistent peer ID
// TODO: encrypt on disk

// NodeKey is the persistent peer key.
// It contains the nodes private key for authentication.
type NodeKey struct {
	// Canonical ID - hex-encoded pubkey's address (IDByteLength bytes)
	ID ID `json:"id"`
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

// PubKeyToID returns the ID corresponding to the given PubKey.
// It's the hex-encoding of the pubKey.Address().
func PubKeyToID(pubKey crypto.PubKey) ID {
	return ID(hex.EncodeToString(pubKey.Address()))
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
		ID:      PubKeyToID(privKey.PubKey()),
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
	nodeKey.ID = PubKeyToID(nodeKey.PubKey())
	return nodeKey, nil
}
