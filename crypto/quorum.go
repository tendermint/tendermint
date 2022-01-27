package crypto

import (
	"github.com/dashevo/dashd-go/btcjson"
	bls "github.com/dashpay/bls-signatures/go-bindings"
)

// SignID returns signing session data that will be signed to get threshold signature share.
// See DIP-0007
func SignID(llmqType btcjson.LLMQType, quorumHash QuorumHash, requestID []byte, messageHash []byte) []byte {
	var blsQuorumHash bls.Hash
	copy(blsQuorumHash[:], quorumHash.Bytes())

	var blsRequestID bls.Hash
	copy(blsRequestID[:], requestID)

	var blsMessageHash bls.Hash
	copy(blsMessageHash[:], messageHash)

	blsSignHash := bls.BuildSignHash(uint8(llmqType), blsQuorumHash, blsRequestID, blsMessageHash)

	signHash := make([]byte, 32)
	copy(signHash, blsSignHash[:])

	return signHash
}
