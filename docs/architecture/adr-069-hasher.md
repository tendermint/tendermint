# ADR 069: hasher 

## Changelog

- 26 July 2021: Initial Draft (@jayt106)

## Status

Proposed

## Context

The current hashing algorithm in the Tendermint core project relies on the Go standard package [sha256](https://pkg.go.dev/crypto/sha256) and the implementation [crypto/tmhash](https://github.com/tendermint/tendermint/blob/master/crypto/tmhash/hash.go) only. The user might want to extend the hash algorithm for the different project's
requirements. For example, the [ethermint](https://github.com/tharsis/ethermint) tries to bring the transaction to the Ethereum network but the transaction hash in the Ethereum world represents it in a different way [RLP hash](https://github.com/ethereum/go-ethereum/blob/92b8f28df3255c6cef9605063850d77b46146763/core/types/transaction.go#L368).

Alternatively, some advanced hash algorithm provides faster-hashing speed, it might increase some scabilities to the project, like the mempool processing, trie proccessing, database data store/retrieve, and so on. For example, the supplementary Go cryptography libraries implemented [blake2b](https://pkg.go.dev/golang.org/x/crypto/blake2b) has roughly 2.25X faster than the `sha256` we mentioned above (both testing under the AVX2 supported CPU processor) [ref](https://github.com/SimonWaldherr/golang-benchmarks#hash). The [sha256-simd](https://github.com/minio/sha256-simd/) provides up to 8x faster than the `crypto/sha256` if the CPU processor supports [AVX512](https://en.wikipedia.org/wiki/AVX-512). The [xxHash](https://github.com/cespare/xxhash) has been using in [Polkadot](https://substrate.dev/docs/en/knowledgebase/advanced/cryptography#hashing-algorithms) project and it provides dramatically speed hashing can be using certain scenarios like the hash of immutable data input.

## Decision
TBD

## Detailed Design

### The new hashing struct
Introduce `Hasher` interface to inherit [hash.Hash](https://pkg.go.dev/hash#Hash) and extend it with the current Tendermint core existing functions. The `Hasher` implementation is required in each supporting hash algorithm.

```go
package tmhash

type Hasher interface {
    hash.Hash
    SumTruncated(bz []byte) []byte
}
```

We would like to define the HashType for using in the later struct.
```go
type HashType string
const (
	SHA256 HashType = "sha256"
	BLAKE2B HashType = "blake2b"
    CUSTOM HashType = "custom"
    // SHA256_SIMD HashType = "sha256_simd"
    // other hash type if the TM core want to support
)

```

The `HasherContainer` provide a container to keep the different `Hasher` instance base on the information
provides when construcing the node.

```go
type HasherContainer struct {
    hasherMap map[HashType]Hasher
    RegisterType(HashType) error
    GetHasher(HashType) (Hasher, error)
}

func (hc HasherContainer) RegisterType(ht HashType, customHasher Hasher) error {
    switch ht {
        case string(tmHash.SHA256):
            hasherMap[ht] := sha256.New()
            return nil
        case string(tmHash.BLAKE2B):
            hasherMap[ht] := blake2b.New()
            return nil
        case string(tmHash.CUSTOM):
            hasherMap[ht] := customhash.New(customHasher)
            return nil
        default:
            return errors.New("register an invalid hash type %w", ht) 
    }
}

func (hc HasherContainer) GetHasher(ht HashType) Hasher {
    return hc.hasherMap[ht]
}

```

### Customizable hashing
Because of we introduce a customize HashType - `CUSTOM`. The developer will need to implement their customize hasher and register it into the `HasherContainer` by calling `RegisterType(CUSTOM, customHasher)`

```go
package tmhash

type CustomHash struct {
    hasher tmHash.Hasher
}

var _ tmHash.Hasher = (*CustomHash)(nil)

func New(hr tmhash.Hasher) tmhash.Hasher {
    return &CustomHash{hasher: hr}
}

func (ch *CustomHash) SumTruncated(bz []byte) []byte {
	return ch.hasher.SumTruncated(bz)
}

// other implementation for the hash.Hash functions
...
``` 

### Chain parameter settings
The hash algorithm is part of the consensus parameters. Therefore, we need to add
new object `hash` into `genesis.json`. There are 2 objects in the `hash` object. `types` will be the hash algorithm be used in the hasher; The `custom_tx_hash_enable` allows the transaction to return its hash using custom hasher. The default value of `types` will be `sha256` and `custom_tx_hash_enable` will be `false` For compatibility, the current `genesis.json` without object `hash` settings will be using the default value above.

```json
{   "chain_id": "tm-chain",
    ...
    "consensus_params": {
        "block": {...},
        "evidence": {...},
        "validator": {...},
        "version": {...},
        "hash": {
            "types": "sha256",
            "custom_tx_hash_enable": "true"
        }
    },
    ...
}
```

### Implementation steps
1. Implement `Hasher`, `HashType`, `HasherContainer`, and `CustomHash` into `tmhash` package.
2. Initialize the `HasherContainer` in the node constructor. Might require passing the hasher to the reactors.
3. Replace the hard-coded tmhash function call using `HasherContainer.GetHasher(SHA256)`.
4. Mock custom hasher and test it.
5. Add new objects into the genesis file, register `Hasher` base on settings, and implement the custom transaction hash.


### Positive
- Support multiple hash algorithms could improve the project scalability.
- Provide the customizable hash algorithm ability for the different projects need.
- The Hasher interface lowers the difficulty to integrate the new hash algorithm in the future. 

### Negative
- More settings when constructing node service.
- Consensus breaks if switching the hash algorithm in the existing network.

### Neutral

## References

- https://github.com/tendermint/tendermint/issues/6539
