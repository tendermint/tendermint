package e2e

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strconv"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/dash/llmq"
)

// proTxHashGenerator generates pseudorandom proTxHash based on a seed.
type proTxHashGenerator struct {
	random *rand.Rand
}

func newProTxHashGenerator(seed int64) *proTxHashGenerator {
	return &proTxHashGenerator{
		random: rand.New(rand.NewSource(seed)), //nolint: gosec
	}
}

func (g *proTxHashGenerator) generate() crypto.ProTxHash {
	seed := make([]byte, crypto.DefaultHashSize)
	_, err := io.ReadFull(g.random, seed)
	if err != nil {
		panic(err) // this shouldn't happen
	}
	return seed
}

// quorumHashGenerator generates pseudorandom quorumHash based on a seed.
type quorumHashGenerator struct {
	random *rand.Rand
}

func newQuorumHashGenerator(seed int64) *quorumHashGenerator {
	return &quorumHashGenerator{
		random: rand.New(rand.NewSource(seed)), //nolint: gosec
	}
}

func (g *quorumHashGenerator) generate() crypto.QuorumHash {
	seed := make([]byte, crypto.DefaultHashSize)

	_, err := io.ReadFull(g.random, seed)
	if err != nil {
		panic(err) // this shouldn't happen
	}
	return seed
}

type initNodeFunc func(*Node) error
type initValidatorFunc func(*Node, crypto.ProTxHash, crypto.QuorumKeys) error

func initValidator(iter *llmq.Iter, quorumHash crypto.QuorumHash, handlers ...initValidatorFunc) initNodeFunc {
	return func(node *Node) error {
		if !iter.Next() {
			return errors.New("unable to move iterator to next an element")
		}
		if node.PrivvalKeys == nil {
			node.PrivvalKeys = make(map[string]crypto.QuorumKeys)
		}
		proTxHash, qks := iter.Value()
		node.PrivvalKeys[quorumHash.String()] = qks
		for _, handler := range handlers {
			err := handler(node, proTxHash, qks)
			if err != nil {
				return err
			}
		}
		return nil
	}
}

func updateProTxHash() initValidatorFunc {
	return func(node *Node, proTxHash crypto.ProTxHash, qks crypto.QuorumKeys) error {
		node.ProTxHash = proTxHash
		fmt.Printf("Setting validator %s proTxHash to %s\n", node.Name, node.ProTxHash.ShortString())
		return nil
	}
}

func updateGenesisValidators(testnet *Testnet) initValidatorFunc {
	return func(node *Node, proTxHash crypto.ProTxHash, qks crypto.QuorumKeys) error {
		vu, err := node.validatorUpdate(qks.PubKey)
		if err != nil {
			return err
		}
		testnet.Validators[node] = ValidatorConfig{vu}
		return nil
	}
}

func updatePrivvalUpdateHeights(height int, quorumHash crypto.QuorumHash) initNodeFunc {
	return func(node *Node) error {
		if height == 0 {
			return nil
		}
		if node.PrivvalUpdateHeights == nil {
			node.PrivvalUpdateHeights = make(map[string]crypto.QuorumHash)
		}
		node.PrivvalUpdateHeights[strconv.Itoa(height)] = quorumHash
		return nil
	}
}

func updateValidatorUpdate(valUpdate ValidatorsMap) initValidatorFunc {
	return func(node *Node, proTxHash crypto.ProTxHash, qks crypto.QuorumKeys) error {
		vu, err := node.validatorUpdate(qks.PubKey)
		if err != nil {
			return err
		}
		valUpdate[node] = ValidatorConfig{vu}
		return nil
	}
}

func initAnyNode(thresholdPublicKey crypto.PubKey, quorumHash crypto.QuorumHash) initNodeFunc {
	return func(node *Node) error {
		quorumKeys := crypto.QuorumKeys{
			ThresholdPublicKey: thresholdPublicKey,
		}
		if node.PrivvalKeys == nil {
			node.PrivvalKeys = make(map[string]crypto.QuorumKeys)
		}
		node.PrivvalKeys[quorumHash.String()] = quorumKeys
		return nil
	}
}

func printInitValidatorInfo(height int) initValidatorFunc {
	return func(node *Node, proTxHash crypto.ProTxHash, qks crypto.QuorumKeys) error {
		if height == 0 {
			fmt.Printf("Set validator %s/%X (at genesis) pubkey to %X\n", node.Name,
				node.ProTxHash, qks.PubKey.Bytes())
			return nil
		}
		fmt.Printf("Set validator %s/%X (at height %d (+ 2)) pubkey to %X\n", node.Name,
			node.ProTxHash, height, qks.PubKey.Bytes())
		return nil
	}
}

func genProTxHashes(proTxHashGen *proTxHashGenerator, n int) []crypto.ProTxHash {
	proTxHashes := make([]crypto.ProTxHash, n)
	for i := 0; i < n; i++ {
		proTxHashes[i] = proTxHashGen.generate()
		if proTxHashes[i] == nil || len(proTxHashes[i]) != crypto.ProTxHashSize {
			panic("the proTxHash must be 32 bytes")
		}
	}
	return proTxHashes
}

func countValidators(nodes map[string]*ManifestNode) int {
	cnt := 0
	for _, node := range nodes {
		nodeManifest := node
		if nodeManifest.Mode == "" || Mode(nodeManifest.Mode) == ModeValidator {
			cnt++
		}
	}
	return cnt
}

func updateNodeParams(nodes []*Node, initFuncs ...initNodeFunc) error {
	// Set up genesis validators. If not specified explicitly, use all validator nodes.
	for _, node := range nodes {
		for _, initFunc := range initFuncs {
			err := initFunc(node)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func makeProTxHashMap(proTxHashes []crypto.ProTxHash) map[string]struct{} {
	m := make(map[string]struct{})
	for _, proTxHash := range proTxHashes {
		m[proTxHash.String()] = struct{}{}
	}
	return m
}

func modifyHeight(height int) int {
	if height > 0 {
		return height + 2
	}
	return height
}

// filter
func nodesFilter(nodes []*Node, cond func(node *Node) bool) []*Node {
	var res []*Node
	for _, node := range nodes {
		if cond(node) {
			res = append(res, node)
		}
	}
	return res
}

func shouldNotBeValidator() func(node *Node) bool {
	return func(node *Node) bool {
		return node.Mode != ModeValidator
	}
}

func shouldHaveName(allowed map[string]int64) func(node *Node) bool {
	return func(node *Node) bool {
		if node.Mode == ModeValidator && len(allowed) == 0 {
			return true
		}
		_, ok := allowed[node.Name]
		return ok
	}
}

func proTxHashShouldNotBeIn(proTxHashes []crypto.ProTxHash) func(node *Node) bool {
	proTxHashesMap := makeProTxHashMap(proTxHashes)
	return func(node *Node) bool {
		flag := false
		if node.ProTxHash != nil {
			_, flag = proTxHashesMap[node.ProTxHash.String()]
		}
		return !flag
	}
}

func lookupNodesByProTxHash(testnet *Testnet, proTxHashes ...crypto.ProTxHash) []*Node {
	res := make([]*Node, 0, len(proTxHashes))
	nodeMap := make(map[string]*Node)
	for _, node := range testnet.Nodes {
		nodeMap[node.ProTxHash.String()] = node
	}
	for _, proTxHash := range proTxHashes {
		res = append(res, nodeMap[proTxHash.String()])
	}
	return res
}
