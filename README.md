# Tendermint DB

Data Base abstractions to be used in applications.
These abstractions are not only meant to be used in applications built on [Tendermint](https://github.com/tendermint/tendermint), but can be used in a variety of applications.

### Minimum Go Version

Go 1.13+

## Supported Databases

- [GolevelDB](https://github.com/syndtr/goleveldb)
- [ClevelDB](https://github.com/google/leveldb)
- [BoltDB](https://github.com/etcd-io/bbolt)
- [GoRocksDB](https://github.com/tecbot/gorocksdb)
- [MemDB](#memdb)
- [RemoteDB](#remotedb)


## Using Databases

### GolevelDB

To use goleveldb there is no need to install anything and you can run the tests by simply running `make test`

### ClevelDB

To use cleveldb leveldb must be installed on your machine. Please use you local package manager (brew, snap) to install the db. Once you have installed it you can run tests with `make test-cleveldb`.

### BoltDB

The BoltDB implementation uses bbolt from etcd.

You can test boltdb by running `make test-boltdb`

### RocksDB

To use RocksDB, you must have it installed on your machine. You can install rocksdb by using your machines package manager (brew, snap). Once you have it installed you can run tests using `make test-rocksdb`.

### MemDB

MemDB is a go implementation of a in memory database. It is mainly used for testing purposes but can be used for other purposes as well. To test the database you can run `make test`.

### RemoteDB

RemoteDB is a database meant for connecting to distributed Tendermint db instances. This can help with detaching difficult deployments such as cleveldb, it can also ease
the burden and cost of deployment of dependencies for databases
to be used by Tendermint developers. It is built with [gRPC](https://grpc.io/). To test this data base you can run `make test`.

If you have all the databases installed on your machine then you can run tests with `make test-all`
