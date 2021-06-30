package p2p

import (
	"io/ioutil"

	"github.com/tendermint/tendermint/internal/p2p"
	tmjson "github.com/tendermint/tendermint/libs/json"
	tmos "github.com/tendermint/tendermint/libs/os"
	"github.com/tendermint/tendermint/types"
)

// LoadNodeKey loads NodeKey located in filePath.
func LoadNodeKeyID(filePath string) (types.NodeID, error) {
	jsonBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	nodeKey := p2p.NodeKey{}
	err = tmjson.Unmarshal(jsonBytes, &nodeKey)
	if err != nil {
		return "", err
	}
	nodeKey.ID = types.NodeIDFromPubKey(nodeKey.PubKey())
	return nodeKey.ID, nil
}

// LoadOrGenNodeKey attempts to load the NodeKey from the given filePath. If
// the file does not exist, it generates and saves a new NodeKey.
func LoadOrGenNodeKeyID(filePath string) (types.NodeID, error) {
	if tmos.FileExists(filePath) {
		nodeKey, err := LoadNodeKeyID(filePath)
		if err != nil {
			return "", err
		}
		return nodeKey, nil
	}

	nodeKey := p2p.GenNodeKey()

	if err := nodeKey.SaveAs(filePath); err != nil {
		return "", err
	}

	return nodeKey.ID, nil
}
