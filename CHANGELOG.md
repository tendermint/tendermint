# Changelog

## Unreleased

## 0.4.1

**2020-2-26**

### Breaking Changes

- [fsdb] [\#43](https://github.com/tendermint/tm-db/pull/43) Remove FSDB

### Bug Fixes

- [boltdb] [\#45](https://github.com/tendermint/tm-db/pull/45) Bring BoltDB to adhere to the db interfaces

## 0.4

**2020-1-7**

### BREAKING CHANGES

- [\#30](https://github.com/tendermint/tm-db/pull/30) Interface Breaking, Interfaces return errors instead of panic:
  - Changes to function signatures:
    - DB interface:
      - `Get([]byte) ([]byte, error)`
      - `Has(key []byte) (bool, error)`
      - `Set([]byte, []byte) error`
      - `SetSync([]byte, []byte) error`
      - `Delete([]byte) error`
      - `DeleteSync([]byte) error`
      - `Iterator(start, end []byte) (Iterator, error)`
      - `ReverseIterator(start, end []byte) (Iterator, error)`
      - `Close() error`
      - `Print() error`
    - Batch interface:
      - `Write() error`
      - `WriteSync() error`
    - Iterator interface:
      - `Error() error`

### IMPROVEMENTS

- [remotedb] [\#34](https://github.com/tendermint/tm-db/pull/34) Add proto file tests and regenerate remotedb.pb.go

## 0.3

**2019-11-18**

Special thanks to external contributors on this release:

### BREAKING CHANGES

- [\#26](https://github.com/tendermint/tm-db/pull/26/files) Rename `DBBackendtype` to `BackendType`

## 0.2

**2019-09-19**

Special thanks to external contributors on this release: @stumble

### Features

- [\#12](https://github.com/tendermint/tm-db/pull/12) Add `RocksDB` (@stumble)

## 0.1

**2019-07-16**

Special thanks to external contributors on this release:

### Initial Release

- `db`, `random.go`, `bytes.go` and `os.go` from the tendermint repo.
