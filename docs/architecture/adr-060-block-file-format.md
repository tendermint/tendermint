# ADR 060: Block File Format

## Changelog

- 2020-09-15: initial version

## Context

The block store currently stores blocks in a database, typically LevelDB. This
is suboptimal, since these databases usually aren't optimized for storage of
BLOBs, operations like LSM-tree compaction becomes prohibitively expensive, and
contents can't be streamed. Blocks should instead be stored in the filesystem,
with metadata and a checksum in the LevelDB database.

## Decision

Store blocks in the filesystem. The whole content of a block will be written to
a file (e.g. `1.block`), where 1 is the block's height. When someone calls
`LoadBlock`, the function should retrieve the content and compare the checksum
to one stored in the database.

```go
type BlockMeta struct {
	BlockID        BlockID  `json:"block_id"`
	BlockSize      int      `json:"block_size"`
	Header         Header   `json:"header"`
	NumTxs         int      `json:"num_txs"`
	Checksum       uint32   `json:"checksum"`
	PartsChecksums []uint32 `json:"parts_checksums"`
}
```

The checksum of a block and its parts will be a part of `BlockMeta`.

`PartsChecksums` are needed because the consensus reactor sometimes calls
`LoadBlockPart` (to help other peers to catch up), which will seek to the given
part location within the file and retrieve it.

NOTE: the part size is always constant (even when we implement erasure coding),
so there's no need to record part size anywhere.

## Alternative Approaches

Store the block's checksum and parts checksums in the same file as the block
itself (along with the version).

## Status

Proposed

## Consequences

### Positive

* No more long pauses due to LevelDB compaction

### Negative

* [files are hard](https://danluu.com/file-consistency/)

### Neutral

## References
