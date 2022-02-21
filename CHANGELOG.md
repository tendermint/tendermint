# Changelog

## 0.6.7

**2022-2-21**

- Use cosmos/rocksdb instead of techbot/rocksdb
- Add `ForceCopmact` to goleveldb database

## 0.6.6

**2021-11-08**

**Important note:** Version v0.6.5 was accidentally tagged and should be
avoided.  This version is identical to v0.6.4 in package structure and API, but
has updated the version marker so that normal `go get` upgrades will not
require modifying existing use of v0.6.4.

### Version bumps (since v0.6.4)

- Bump grpc from to 1.42.0.
- Bump dgraph/badger to v2 2.2007.3.
- Bump go.etcd.io/bbolt to 1.3.6.

## 0.6.5

**2021-08-04**

**Important note**: This version was tagged by accident, and should not be
used. The tag now points to the [package-reorg
branch](https://github.com/tendermint/tm-db/tree/package-reorg) so that
any existing dependencies should not break.

## 0.6.4

**2021-02-09**

Bump protobuf to 1.3.2 and grpc to 1.35.0.

## 0.6.3

**2020-11-10**

### Improvements

- [goleveldb] [\#134](https://github.com/tendermint/tm-db/pull/134) Improve iterator performance by bounding underlying iterator range (@klim0v)

## 0.6.2

**2020-08-27**

Bump grpc, badger and goleveldb (see versions in go.mod file)

## 0.6.1

**2020-08-12**

### Improvements

- [\#115](https://github.com/tendermint/tm-db/pull/115) Add a `BadgerDB` backend with build tag `badgerdb` (@mvdan)

## 0.6.0

**2020-06-24**

### Breaking Changes

- [\#99](https://github.com/tendermint/tm-db/pull/99) Keys can no longer be `nil` or empty, and values can no longer be `nil` (@erikgrinaker)

- [\#98](https://github.com/tendermint/tm-db/pull/98) `NewDB` now returns an error instead of panicing (@erikgrinaker)

- [\#96](https://github.com/tendermint/tm-db/pull/96) `Batch.Set()`, `Delete()`, and `Close()` may now error (@erikgrinaker)

- [\#97](https://github.com/tendermint/tm-db/pull/97) `Iterator.Close()` may now error (@erikgrinaker)

- [\#97](https://github.com/tendermint/tm-db/pull/97) Many iterator panics are now exposed via `Error()` instead (@erikgrinaker)

- [\#96](https://github.com/tendermint/tm-db/pull/96) The `SetDeleter` interface has been removed (@erikgrinaker)

### Bug Fixes

- [\#97](https://github.com/tendermint/tm-db/pull/97) `RemoteDB` iterators are now correctly primed with the first item when created, without calling `Next()` (@erikgrinaker)

## 0.5.2

**2020-11-10**

### Improvements

- [goleveldb] [\#134](https://github.com/tendermint/tm-db/pull/134) Improve iterator performance by bounding underlying iterator range (@klim0v)

## 0.5.1

**2020-03-30**

### Bug Fixes

- [boltdb] [\#81](https://github.com/tendermint/tm-db/pull/81) Use correct import path go.etcd.io/bbolt

## 0.5.0

**2020-03-11**

### Breaking Changes

- [\#71](https://github.com/tendermint/tm-db/pull/71) Closed or written batches can no longer be reused, all non-`Close()` calls will panic

- [memdb] [\#74](https://github.com/tendermint/tm-db/pull/74) `Iterator()` and `ReverseIterator()` now take out database read locks for the duration of the iteration

- [memdb] [\#56](https://github.com/tendermint/tm-db/pull/56) Removed some exported methods that were mainly meant for internal use: `Mutex()`, `SetNoLock()`, `SetNoLockSync()`, `DeleteNoLock()`, and `DeleteNoLockSync()`

### Improvements

- [memdb] [\#53](https://github.com/tendermint/tm-db/pull/53) Use a B-tree for storage, which significantly improves range scan performance

- [memdb] [\#56](https://github.com/tendermint/tm-db/pull/56) Use an RWMutex for improved performance with highly concurrent read-heavy workloads

### Bug Fixes

- [boltdb] [\#69](https://github.com/tendermint/tm-db/pull/69) Properly handle blank keys in iterators

- [cleveldb] [\#65](https://github.com/tendermint/tm-db/pull/65) Fix handling of empty keys as iterator endpoints

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
