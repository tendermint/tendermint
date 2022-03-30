module github.com/tendermint/tendermint

go 1.16

require (
	github.com/BurntSushi/toml v1.0.0
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/Workiva/go-datastructures v1.0.53
	github.com/adlio/schema v1.3.0
	github.com/btcsuite/btcd v0.22.0-beta
	github.com/btcsuite/btcutil v1.0.3-0.20201208143702-a53e38424cce
	github.com/containerd/continuity v0.2.2 // indirect
	github.com/dashevo/dashd-go v0.23.4
	github.com/dashevo/dashd-go/btcec/v2 v2.0.6 // indirect
	github.com/dashpay/bls-signatures/go-bindings v0.0.0-20201127091120-745324b80143
	github.com/dgraph-io/badger/v2 v2.2007.4 // indirect
	github.com/dgraph-io/ristretto v0.1.0 // indirect
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/didip/tollbooth/v6 v6.1.2 // indirect
	github.com/didip/tollbooth_chi v0.0.0-20200828173446-a7173453ea21 // indirect
	github.com/facebookgo/ensure v0.0.0-20200202191622-63f1cf65ac4c // indirect
	github.com/facebookgo/stack v0.0.0-20160209184415-751773369052 // indirect
	github.com/facebookgo/subset v0.0.0-20200203212716-c811ad88dec4 // indirect
	github.com/fortytw2/leaktest v1.3.0
	github.com/go-chi/chi v4.1.2+incompatible // indirect
	github.com/go-kit/kit v0.12.0
	github.com/go-pkgz/jrpc v0.2.0
	github.com/go-pkgz/rest v1.13.0 // indirect
	github.com/gogo/protobuf v1.3.2
	github.com/golang/glog v1.0.0 // indirect
	github.com/golang/protobuf v1.5.2
	github.com/golang/snappy v0.0.4 // indirect
	github.com/golangci/golangci-lint v1.44.0
	github.com/google/btree v1.0.1 // indirect
	github.com/google/orderedcode v0.0.1
	github.com/google/uuid v1.3.0
	github.com/gorilla/websocket v1.5.0
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1
	github.com/klauspost/compress v1.15.1 // indirect
	github.com/lib/pq v1.10.4
	github.com/libp2p/go-buffer-pool v0.0.2
	github.com/magiconair/properties v1.8.6 // indirect
	github.com/minio/highwayhash v1.0.2
	github.com/mroth/weightedrand v0.4.1
	github.com/oasisprotocol/curve25519-voi v0.0.0-20220328075252-7dd334e3daae
	github.com/opencontainers/runc v1.1.1 // indirect
	github.com/ory/dockertest v3.3.5+incompatible
	github.com/petermattis/goid v0.0.0-20220302125637-5f11c28912df // indirect
	github.com/prometheus/client_golang v1.12.1
	github.com/prometheus/common v0.33.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475
	github.com/rs/cors v1.8.2
	github.com/rs/zerolog v1.26.1
	github.com/sasha-s/go-deadlock v0.3.1
	github.com/snikch/goodman v0.0.0-20171125024755-10e37e294daa
	github.com/spf13/afero v1.8.2 // indirect
	github.com/spf13/cobra v1.4.0
	github.com/spf13/viper v1.10.1
	github.com/stretchr/objx v0.3.0 // indirect
	github.com/stretchr/testify v1.7.1
	github.com/tendermint/tm-db v0.6.6
	github.com/vektra/mockery/v2 v2.10.0
	golang.org/x/crypto v0.0.0-20220321153916-2c7772ba3064
	golang.org/x/net v0.0.0-20220325170049-de3da57026de
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	golang.org/x/sys v0.0.0-20220330033206-e17cdc41300f // indirect
	golang.org/x/time v0.0.0-20220224211638-0e9765cccd65 // indirect
	google.golang.org/genproto v0.0.0-20220329172620-7be39ac1afc7 // indirect
	google.golang.org/grpc v1.45.0
	gopkg.in/check.v1 v1.0.0-20200902074654-038fdea0a05b // indirect
	gopkg.in/ini.v1 v1.66.4 // indirect
	pgregory.net/rapid v0.4.7
)

replace github.com/tendermint/tendermint => ./
