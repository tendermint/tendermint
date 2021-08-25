# ADR 070: hasher 

## Changelog
- 25 Augest 2021: Revision (@jayt106)
- 26 July 2021: Initial Draft (@jayt106)

## Status
Proposed

## Context
The current hashing algorithm in the Tendermint core project relies on the Go standard package [sha256](https://pkg.go.dev/crypto/sha256) and the implementation [crypto/tmhash](https://github.com/tendermint/tendermint/blob/master/crypto/tmhash/hash.go) only. The user might want to extend the hash algorithm for the different project's requirements. For example, the [ethermint](https://github.com/tharsis/ethermint) tries to bring the transaction to the Ethereum network but the transaction hash in the Ethereum world represents it in a different way [RLP hash](https://github.com/ethereum/go-ethereum/blob/92b8f28df3255c6cef9605063850d77b46146763/core/types/transaction.go#L368).

Alternatively, some advanced hash algorithm provides faster-hashing speed, it might increase the scalability to the project, like the mempool processing, trie processing, database data store/retrieve, and so on. For example, the supplementary Go cryptography libraries implemented [blake2b](https://pkg.go.dev/golang.org/x/crypto/blake2b) has roughly 2.25X faster than the `sha256` we mentioned above (both testing under the AVX2 supported CPU processor) [ref](https://github.com/SimonWaldherr/golang-benchmarks#hash). The [sha256-simd](https://github.com/minio/sha256-simd/) provides up to 8x faster than the `crypto/sha256` if the CPU processor supports [AVX512](https://en.wikipedia.org/wiki/AVX-512). The [xxHash](https://github.com/cespare/xxhash) has been using in [Polkadot](https://substrate.dev/docs/en/knowledgebase/advanced/cryptography#hashing-algorithms) project and it provides dramatically speed hashing can be using certain scenarios like the hash of immutable data input.

Some previous discussions like [#5632](https://github.com/tendermint/tendermint/issues/5631), would like to use the different hash algorithm into the Merkle tree without forking the repo. [#2186](https://github.com/tendermint/tendermint/issues/2186) and [#2187](https://github.com/tendermint/tendermint/issues/2187) would like to use a faster hash to speed up the project. 

In the SDK, [blake3](https://github.com/BLAKE3-team/BLAKE3) is also being considered as a hasher option. Therefore, we can integrate `blake3` if Tendermint and the SDK want to use the same hash package, i.e. `tmhash`.

## Decision
TBD

## Detailed Design
To tackle these issues, we can separate into two directions: configurable global hasher, and the custom transaction hash with `hasher` injection.

### Configuable global Hashers
Some of the components like: `merkle`, `evidence`, `tx`, and `maps` in SDK rely on the `tmhash.Sum()`, we would like to propose using configurable global Hashers instead of it. The `Hashers` provides a container that can store the different hash algorithms. Therefore, developers can assign the `custom hasher` before running the services. 

```go
package tmhash

type HashType string
const (
    SHA256 HashType = "sha256"
    BLAKE2B HashType = "blake2b"
    // SHA256_SIMD HashType = "sha256_simd"
    // other hash type if the Tendermint would like to support by default
)

// A globle hashers
var Hashers *HasherContainer

type HasherContainer struct {
	mainHasher HashType // it indicates which hasher is mainly using in the tendermint components.
	hasherMap  map[HashType]hash.Hash
}

func init() {
	Hashers = initContainer()
}

func initContainer() *HasherContainer {
	hc := &HasherContainer{
		hasherMap: map[HashType]hash.Hash{}}

    // during the hashers initialization, it will adds all hasher have been integreted in 
    // Tendermint crypto library by default. And the mainHasher will be sha256 by default.    
	hc.RegisterHasher(SHA256, sha256.New(), true)
	hc.RegisterHasher(BLAKE2B, blake2b.New(), false)

	return hc
}

// develop can register the custom hasher into the hashers and set it to the main uses.
func (hc *HasherContainer) RegisterHasher(ht HashType, hasher hash.Hash, isMain bool) {
	hc.hasherMap[ht] = hasher
	if isMain {
		hc.mainHasher = ht
	}
}

func (hc *HasherContainer) MainHasher() hash.Hash {
	return hc.hasherMap[hc.mainHasher]
}

func (hc *HasherContainer) MainHashType() HashType {
	return hc.mainHasher
}

func (hc *HasherContainer) Hasher(ht HashType) hash.Hash {
	return hc.hasherMap[ht]
}

func (hc *HasherContainer) SetMainHashType(ht HashType) error {
    if hc.hasherMap[ht] == nil {
        return err
    }

	return nil
}
```

### Custom transaction hash
The developer might want to use the custom hasher for fulfilling the special requirement, like [#6539](https://github.com/tendermint/tendermint/issues/6539). We would like to implement another `tx.Hash()` in `types/tx.go` function to allow the custom hasher injection when calculates the transaction hash. Therefore, the project construct on top of Tendermint can call this function directly without any changes.

```go
func (tx TX) Hash(hasher hash.Hash) []byte {
    return hasher.Sum(tx)
}
```

### Chain parameter settings
For the project built on top of Tendermint can use `Hashers.RegisterHasher` to change the main hasher it would like to use. For Tendermint itself, it requires a chain parameter to indicate which hasher should be used in the service when the beginning. Therefore, we need to add a `hasher` param into the `genesis.json`.

```json
{   "chain_id": "tm-chain",
    ...
    "consensus_params": {
        "block": {...},
        "evidence": {...},
        "validator": {...},
        "version": {...},
    },
    ...
    "hasher" : "sha256",
    ...
}
```

### Implementation steps
1. Adds `tx.Hash(h hash.Hash)` function in `types/tx.go`, can be a small PR and benefit [#6539](https://github.com/tendermint/tendermint/issues/6539).
2. Decides which hasher should be integrated by default.
3. Implements configurable global Hashers.
4. Replaces the hard-coded tmhash call like `tmhash.Sum()` with `tmhash.Hashers.MainHasher().Sum()`

### Positive
- Supports multiple hash algorithms that could improve the project scalability.
- Provides the customizable hash algorithm ability for the different projects need.
- The Hasher interface lowers the difficulty to integrate the new hash algorithm in the future. 

### Negative
- Must be careful when calling `RegisterHasher` and `SetMainHashType`, it will change the hash behavior of the whole project. Only calling it when constructing the services.
- Consensus breaks if switching the hash algorithm in the existing network.

### Neutral
- If we keep the original tmhash function like `tmhash.Sum()`, it's not a breaking change to the project built on top of Tendermint.

## References
- [modular transaction hashing](https://github.com/tendermint/tendermint/issues/6539)
- [crypto: Switch tmhash to AVX2 sped up SHA2](https://github.com/tendermint/tendermint/issues/2186)
- [Make the tree to merkelize Txs pluggable or at least the underlying hash](https://github.com/tendermint/tendermint/issues/5631)
- [mempool cache: use a fast hash](https://github.com/tendermint/tendermint/issues/2187)
- [proposal: Genesis Params](https://github.com/tendermint/tendermint/issues/6814)