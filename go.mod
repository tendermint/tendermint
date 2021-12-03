module github.com/tendermint/tm-db

go 1.16

require (
	github.com/dgraph-io/badger/v2 v2.2007.2
	github.com/gogo/protobuf v1.3.2
	github.com/google/btree v1.0.0
	github.com/jmhodges/levigo v1.0.0
	github.com/stretchr/testify v1.7.0
	github.com/syndtr/goleveldb v1.0.1-0.20200815110645-5c35d600f0ca
	github.com/tecbot/gorocksdb v0.0.0-20191217155057-f0fad39f321c
	go.etcd.io/bbolt v1.3.6
	google.golang.org/grpc v1.42.0
)

// Breaking changes were released with the wrong tag (use v0.6.6 or later).
retract v0.6.5

// Note: gorocksdb bindings for OptimisticTransactionDB are not merged upstream, so we use a fork
// See https://github.com/tecbot/gorocksdb/pull/216
replace github.com/tecbot/gorocksdb => github.com/cosmos/gorocksdb v1.1.1
