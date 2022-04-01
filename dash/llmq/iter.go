package llmq

import "github.com/tendermint/tendermint/crypto"

// Iter creates and returns LLMQ iterator
func (l *Data) Iter() *Iter {
	return &Iter{data: l}
}

// Iter is an iterator implementation
// this iterator go through the proTxHash and validator quorum-keys (like: private and public share keys)
type Iter struct {
	proTxHash  crypto.ProTxHash
	quorumKeys crypto.QuorumKeys
	data       *Data
	pos        int
}

// Value returns node's crypto.ProTxHash and crypto.QuorumKeys from the current position
func (i *Iter) Value() (crypto.ProTxHash, crypto.QuorumKeys) {
	return i.proTxHash, i.quorumKeys
}

// Next moves a position pointer to a next element
func (i *Iter) Next() bool {
	if i.pos >= len(i.data.ProTxHashes) {
		return false
	}
	i.proTxHash = i.data.ProTxHashes[i.pos]
	i.quorumKeys = crypto.QuorumKeys{
		PrivKey:            i.data.PrivKeyShares[i.pos],
		PubKey:             i.data.PubKeyShares[i.pos],
		ThresholdPublicKey: i.data.ThresholdPubKey,
	}
	i.pos++
	return true
}
