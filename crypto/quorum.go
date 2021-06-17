package crypto

import (
	"github.com/dashevo/dashd-go/btcjson"
	bls "github.com/dashpay/bls-signatures/go-bindings"
)

func SignId(llmqType btcjson.LLMQType, quorumHash QuorumHash, requestId []byte, messageHash []byte) []byte {
	var blsQuorumHash bls.Hash
	copy(blsQuorumHash[:], quorumHash.Bytes())

	var blsRequestId bls.Hash
	copy(blsRequestId[:], requestId)

	var blsMessageHash bls.Hash
	copy(blsMessageHash[:], messageHash)

	blsSignHash := bls.BuildSignHash(uint8(llmqType), blsQuorumHash, blsRequestId, blsMessageHash)

	signHash := make([]byte, 32)
	copy(signHash, blsSignHash[:])

	return signHash
}
