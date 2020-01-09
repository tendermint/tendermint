package types

// BlockchainState captures status information from the BlockchainReactor
// The interface is implemented in the blockchain package
type BlockchainState interface {
	GetMaxPeerHeight() int64
}
