module github.com/tendermint/tendermint

go 1.15

require (
	github.com/BurntSushi/toml v1.0.0
	github.com/Workiva/go-datastructures v1.0.53
	github.com/adlio/schema v1.2.3
	github.com/btcsuite/btcd v0.22.0-beta
	github.com/btcsuite/btcutil v1.0.3-0.20201208143702-a53e38424cce
	github.com/dashevo/dashd-go v0.0.0-20210630125816-b417ad8eb165
	github.com/dashpay/bls-signatures/go-bindings v0.0.0-20201127091120-745324b80143
	github.com/dgraph-io/badger/v2 v2.2007.3 // indirect
	github.com/didip/tollbooth/v6 v6.1.1 // indirect
	github.com/didip/tollbooth_chi v0.0.0-20200828173446-a7173453ea21 // indirect
	github.com/fortytw2/leaktest v1.3.0
	github.com/go-chi/chi v4.1.2+incompatible // indirect
	github.com/go-kit/kit v0.12.0
	github.com/go-kit/log v0.2.0
	github.com/go-logfmt/logfmt v0.5.1
	github.com/go-pkgz/jrpc v0.2.0
	github.com/go-pkgz/rest v1.11.0 // indirect
	github.com/gogo/protobuf v1.3.2
	github.com/golang/protobuf v1.5.2
	github.com/google/btree v1.0.1 // indirect
	github.com/google/gopacket v1.1.19 // indirect
	github.com/google/orderedcode v0.0.1
	github.com/gorilla/websocket v1.4.2
	github.com/gtank/merlin v0.1.1
	github.com/lib/pq v1.10.4
	github.com/libp2p/go-buffer-pool v0.0.2
	github.com/mimoo/StrobeGo v0.0.0-20210601165009-122bf33a46e0 // indirect
	github.com/minio/highwayhash v1.0.2
	github.com/ory/dockertest v3.3.5+incompatible
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.12.0
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	github.com/rs/cors v1.8.2
	github.com/sasha-s/go-deadlock v0.3.1
	github.com/snikch/goodman v0.0.0-20171125024755-10e37e294daa
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.9.0
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/tendermint/tm-db v0.6.4
	go.etcd.io/bbolt v1.3.6 // indirect
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	golang.org/x/net v0.0.0-20211208012354-db4efeb81f4b
	google.golang.org/genproto v0.0.0-20210929214142-896c89f843d2 // indirect
	google.golang.org/grpc v1.41.0
)

replace github.com/tendermint/tendermint => ./
