package types

import (
	"github.com/tendermint/tendermint/crypto"
	cryptoenc "github.com/tendermint/tendermint/crypto/encoding"
	"github.com/tendermint/tendermint/dash/llmq"
)

// QuorumOptionFunc is an option function for quorum config
type QuorumOptionFunc func(conf *quorumConfig)

type quorumConfig struct {
	nodeAddrs  []string
	power      int64
	quorumHash crypto.QuorumHash
}

// WithNodeAddrs sets node-addresses using option function
func WithNodeAddrs(addrs []string) QuorumOptionFunc {
	return func(conf *quorumConfig) {
		conf.nodeAddrs = addrs
	}
}

// WithPower sets vote-power using option function
func WithPower(power int64) QuorumOptionFunc {
	return func(conf *quorumConfig) {
		conf.power = power
	}
}

// WithRandQuorumHash generates and sets a quorum-hash through option QuorumOptionFunc function
func WithRandQuorumHash() QuorumOptionFunc {
	return WithQuorumHash(crypto.RandQuorumHash())
}

// WithQuorumHash sets a quorum-hash through option QuorumOptionFunc function
func WithQuorumHash(quorumHash crypto.QuorumHash) QuorumOptionFunc {
	return func(conf *quorumConfig) {
		conf.quorumHash = quorumHash
	}
}

// LLMQToValidatorSetProto returns a protobuf validator-set-update structure for passed llmq-data
// use option-functions to override default values like power or node-addresses
func LLMQToValidatorSetProto(ld llmq.Data, opts ...QuorumOptionFunc) (*ValidatorSetUpdate, error) {
	conf := quorumConfig{
		power: 100,
	}
	for _, opt := range opts {
		opt(&conf)
	}
	tpk, err := cryptoenc.PubKeyToProto(ld.ThresholdPubKey)
	if err != nil {
		return nil, err
	}
	vsu := ValidatorSetUpdate{
		ValidatorUpdates: ValidatorUpdatesProto(
			ld.ProTxHashes,
			ld.PubKeyShares,
			conf.nodeAddrs,
			conf.power,
		),
		ThresholdPublicKey: tpk,
		QuorumHash:         conf.quorumHash,
	}
	return &vsu, nil
}

// UpdateValidatorProto returns a protobuf validator-update struct with passed data
func UpdateValidatorProto(
	proTxHash crypto.ProTxHash,
	pubKey crypto.PubKey,
	power int64,
	nodeAddress string,
) ValidatorUpdate {
	valUpdate := ValidatorUpdate{
		Power:       power,
		ProTxHash:   proTxHash,
		NodeAddress: nodeAddress,
	}
	if len(pubKey.Bytes()) > 0 {
		pubKeyProto := cryptoenc.MustPubKeyToProto(pubKey)
		valUpdate.PubKey = &pubKeyProto
	}
	return valUpdate
}

// ValidatorUpdatesProto creates and returns a slice of protobuf update-validator struct
// if number elements in pubKeys are less than in proTxHashes, then for those the elements will be created
// validator-update with power=0 and public-key=nil
func ValidatorUpdatesProto(
	proTxHashes []crypto.ProTxHash,
	pubKeys []crypto.PubKey,
	nodeAddrs []string,
	power int64,
) []ValidatorUpdate {
	var (
		nodeAddr string
		pubKey   crypto.PubKey
	)
	vals := make([]ValidatorUpdate, 0, len(proTxHashes))
	for i, proTxHash := range proTxHashes {
		nodeAddr = ""
		pubKey = nil
		if i < len(nodeAddrs) {
			nodeAddr = nodeAddrs[i]
		}
		if i < len(pubKeys) {
			pubKey = pubKeys[i]
		}
		vals = append(vals, UpdateValidatorProto(proTxHash, pubKey, power, nodeAddr))
	}
	return vals
}
