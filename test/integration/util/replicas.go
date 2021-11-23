package util

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ds-test-framework/scheduler/types"
	"github.com/tendermint/tendermint/crypto"
	tmjson "github.com/tendermint/tendermint/libs/json"
	"github.com/tendermint/tendermint/privval"
)

type ReplicaSet struct {
	replicas map[types.ReplicaID]bool
	size     int
	mtx      *sync.Mutex
}

func NewReplicaSet() *ReplicaSet {
	return &ReplicaSet{
		replicas: make(map[types.ReplicaID]bool),
		size:     0,
		mtx:      new(sync.Mutex),
	}
}

func (r *ReplicaSet) Add(id types.ReplicaID) {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	_, ok := r.replicas[id]
	if !ok {
		r.replicas[id] = true
		r.size = r.size + 1
	}
}

func (r *ReplicaSet) Size() int {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	return r.size
}

func (r *ReplicaSet) Exists(id types.ReplicaID) bool {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	_, ok := r.replicas[id]
	return ok
}

func (r *ReplicaSet) Iter() []types.ReplicaID {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	result := make([]types.ReplicaID, len(r.replicas))
	i := 0
	for r := range r.replicas {
		result[i] = r
		i = i + 1
	}
	return result
}

func (r *ReplicaSet) String() string {
	str := ""
	r.mtx.Lock()
	defer r.mtx.Unlock()

	for r := range r.replicas {
		str += string(r) + ","
	}
	return str
}

// Should cache key instead of decoding everytime
func GetPrivKey(r *types.Replica) (crypto.PrivKey, error) {
	pK, ok := r.Info["privkey"]
	if !ok {
		return nil, errors.New("no private key specified")
	}
	pKS, ok := pK.(string)
	if !ok {
		return nil, errors.New("malformed key type")
	}

	privKey := privval.FilePVKey{}
	err := tmjson.Unmarshal([]byte(pKS), &privKey)
	if err != nil {
		return nil, fmt.Errorf("malformed key: %#v. error: %s", r.Info, err)
	}
	return privKey.PrivKey, nil
}

func GetChainID(r *types.Replica) (string, error) {
	chain_id, ok := r.Info["chain_id"]
	if !ok {
		return "", errors.New("chain id does not exist")
	}
	return chain_id.(string), nil
}
