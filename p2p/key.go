package p2p

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"

	crypto "github.com/tendermint/go-crypto"
	cmn "github.com/tendermint/tmlibs/common"
)

//------------------------------------------------------------------------------
// Persistent peer ID
// TODO: encrypt on disk

// NodeKey is the persistent peer key.
// It contains the nodes private key for authentication.
type NodeKey struct {
	PrivKey crypto.PrivKey `json:"priv_key"` // our priv key
}

type ID string

// ID returns the peer's canonical ID - the hash of its public key.
func (nodeKey *NodeKey) ID() ID {
	return ID(hex.EncodeToString(nodeKey.id()))
}

func (nodeKey *NodeKey) id() []byte {
	return nodeKey.PrivKey.PubKey().Address()
}

// PubKey returns the peer's PubKey
func (nodeKey *NodeKey) PubKey() crypto.PubKey {
	return nodeKey.PrivKey.PubKey()
}

func (nodeKey *NodeKey) SatisfiesTarget(target []byte) bool {
	return bytes.Compare(nodeKey.id(), target) < 0
}

// LoadOrGenNodeKey attempts to load the NodeKey from the given filePath,
// and checks that the corresponding ID is less than the target.
// If the file does not exist, it generates and saves a new NodeKey
// with ID less than target.
func LoadOrGenNodeKey(filePath string, target []byte) (*NodeKey, error) {
	if cmn.FileExists(filePath) {
		nodeKey, err := loadNodeKey(filePath)
		if err != nil {
			return nil, err
		}
		if !nodeKey.SatisfiesTarget(target) {
			return nil, fmt.Errorf("Loaded ID (%s) does not satisfy target (%X)", nodeKey.ID(), target)
		}
		return nodeKey, nil
	} else {
		return genNodeKey(filePath, target)
	}
}

// MakePoWTarget returns a 20 byte target byte array.
func MakePoWTarget(difficulty uint8) []byte {
	zeroPrefixLen := (int(difficulty) / 8)
	prefix := bytes.Repeat([]byte{0}, zeroPrefixLen)
	mod := (difficulty % 8)
	if mod > 0 {
		nonZeroPrefix := byte(1 << (8 - mod))
		prefix = append(prefix, nonZeroPrefix)
	}
	return append(prefix, bytes.Repeat([]byte{255}, 20-len(prefix))...)
}

func loadNodeKey(filePath string) (*NodeKey, error) {
	jsonBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	nodeKey := new(NodeKey)
	err = json.Unmarshal(jsonBytes, nodeKey)
	if err != nil {
		return nil, fmt.Errorf("Error reading NodeKey from %v: %v\n", filePath, err)
	}
	return nodeKey, nil
}

func genNodeKey(filePath string, target []byte) (*NodeKey, error) {
	privKey := genPrivKeyEd25519PoW(target).Wrap()
	nodeKey := &NodeKey{
		PrivKey: privKey,
	}

	jsonBytes, err := json.Marshal(nodeKey)
	if err != nil {
		return nil, err
	}
	err = ioutil.WriteFile(filePath, jsonBytes, 0600)
	if err != nil {
		return nil, err
	}
	return nodeKey, nil
}

// generate key with address satisfying the difficult target
func genPrivKeyEd25519PoW(target []byte) crypto.PrivKeyEd25519 {
	secret := crypto.CRandBytes(32)
	var privKey crypto.PrivKeyEd25519
	for i := 0; ; i++ {
		privKey = crypto.GenPrivKeyEd25519FromSecret(secret)
		if bytes.Compare(privKey.PubKey().Address(), target) < 0 {
			break
		}
		z := new(big.Int)
		z.SetBytes(secret)
		z = z.Add(z, big.NewInt(1))
		secret = z.Bytes()

	}
	return privKey
}
