# State

## State

The state contains information whose cryptographic digest is included in block headers, and thus is
necessary for validating new blocks. For instance, the validators set and the results of
transactions are never included in blocks, but their Merkle roots are - the state keeps track of them.

Note that the `State` object itself is an implementation detail, since it is never
included in a block or gossiped over the network, and we never compute
its hash. Thus we do not include here details of how the `State` object is
persisted or queried. That said, the types it contains are part of the specification, since
their Merkle roots are included in blocks and their values are used in
validation.

```go
type State struct {
    ChainID        string
    InitialHeight  int64

    Version     Version
    LastResults []Result
    AppHash     []byte

    LastValidators []Validator
    Validators     []Validator
    NextValidators []Validator

    ConsensusParams ConsensusParams
}
```

The chain ID and initial height are taken from the genesis file, and not changed again. The
initial height will be `1` in the typical case, `0` is an invalid value.

Note there is a hard-coded limit of 10000 validators. This is inherited from the
limit on the number of votes in a commit.

### Version

```go
type Version struct {
  consensus Consensus
  software string
}
```

The `Consensus` contains the protocol version for the blockchain and the
application as two `uint64` values:

```go
type Consensus struct {
 Block uint64
 App   uint64
}
```

### ResponseDeliverTx

```protobuf
message ResponseDeliverTx {
  uint32         code       = 1;
  bytes          data       = 2;
  string         log        = 3;  // nondeterministic
  string         info       = 4;  // nondeterministic
  int64          gas_wanted = 5;
  int64          gas_used   = 6;
  repeated Event events     = 7
      [(gogoproto.nullable) = false, (gogoproto.jsontag) = "events,omitempty"];
  string codespace = 8;
}
```

`ResponseDeliverTx` is the result of executing a transaction against the application.
It returns a result code (`uint32`), an arbitrary byte array (`[]byte`) (ie. a return value), Log (`string`), Info (`string`), GasWanted (`int64`), GasUsed (`int64`), Events (`[]Events`) and a Codespace (`string`).

### Validator

A validator is an active participant in the consensus with a public key and a voting power.
Validator's also contain an address field, which is a hash digest of the PubKey.

```go
type Validator struct {
    Address     []byte
    PubKey      PubKey
    VotingPower int64
}
```

When hashing the Validator struct, the address is not included,
because it is redundant with the pubkey.

The `state.Validators`, `state.LastValidators`, and `state.NextValidators`,
must always be sorted by voting power, so that there is a canonical order for
computing the MerkleRoot.

We also define a `TotalVotingPower` function, to return the total voting power:

```go
func TotalVotingPower(vals []Validators) int64{
    sum := 0
    for v := range vals{
        sum += v.VotingPower
    }
    return sum
}
```

### ConsensusParams

ConsensusParams define various limits for blockchain data structures.
Like validator sets, they are set during genesis and can be updated by the application through ABCI.
When hashed, only a subset of the params are included, to allow the params to
evolve without breaking the header.

```protobuf
message ConsensusParams {
  BlockParams     block     = 1;
  EvidenceParams  evidence  = 2;
  ValidatorParams validator = 3;
  VersionParams   version   = 4;
}
```

```go
type hashedParams struct {
 BlockMaxBytes int64
 BlockMaxGas   int64
}

func HashConsensusParams() []byte {
 SHA256(hashedParams{
  BlockMaxBytes: params.Block.MaxBytes,
  BlockMaxGas:   params.Block.MaxGas,
 })
}
```

```protobuf
message BlockParams {
  int64 max_bytes = 1;
  int64 max_gas = 2;
  int64 time_iota_ms = 3; // not exposed to the application
}

message EvidenceParams {
  int64 max_age_num_blocks = 1;
  google.protobuf.Duration max_age_duration = 2;
  uint32 max_num = 3;
}

message ValidatorParams {
  repeated string pub_key_types = 1;
}

message VersionParams {
  uint64 AppVersion = 1;
}
```

#### Block

The total size of a block is limited in bytes by the `ConsensusParams.Block.MaxBytes`.
Proposed blocks must be less than this size, and will be considered invalid
otherwise.

Blocks should additionally be limited by the amount of "gas" consumed by the
transactions in the block, though this is not yet implemented.

The minimal time between consecutive blocks is controlled by the
`ConsensusParams.Block.TimeIotaMs`.

#### Evidence

For evidence in a block to be valid, it must satisfy:

```go
block.Header.Time-evidence.Time < ConsensusParams.Evidence.MaxAgeDuration &&
 block.Header.Height-evidence.Height < ConsensusParams.Evidence.MaxAgeNumBlocks
```

#### Validator

Validators from genesis file and `ResponseEndBlock` must have pubkeys of type ∈
`ConsensusParams.Validator.PubKeyTypes`.
