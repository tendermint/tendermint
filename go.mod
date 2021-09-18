module github.com/tendermint/tendermint

go 1.13

require (
	github.com/ChainSafe/go-schnorrkel v0.0.0-20200405005733-88cbf1b4c40d
	github.com/Workiva/go-datastructures v1.0.52
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/btcsuite/btcutil v1.0.2
	github.com/enigmampc/btcutil v1.0.3-0.20200723161021-e2fb6adb2a25
	github.com/fortytw2/leaktest v1.3.0
	github.com/go-kit/kit v0.10.0
	github.com/go-logfmt/logfmt v0.5.0
	github.com/gogo/protobuf v1.3.1
	github.com/golang/protobuf v1.4.0
	github.com/gorilla/websocket v1.4.2
	github.com/gtank/merlin v0.1.1
	github.com/libp2p/go-buffer-pool v0.0.2
	github.com/magiconair/properties v1.8.1
	github.com/minio/highwayhash v1.0.0
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.5.1
	github.com/rcrowley/go-metrics v0.0.0-20200313005456-10cdbea86bc0
	github.com/rs/cors v1.7.0
	github.com/snikch/goodman v0.0.0-20171125024755-10e37e294daa
	github.com/spf13/cobra v1.0.0
	github.com/spf13/viper v1.6.3
	github.com/stretchr/testify v1.5.1
	github.com/tendermint/go-amino v0.15.1
	github.com/tendermint/tm-db v0.5.2
	golang.org/x/crypto v0.0.0-20200709230013-948cd5f35899
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e
	google.golang.org/grpc v1.29.1
)

replace github.com/tendermint/tm-db => github.com/okex/tm-db v0.5.2-exchain1
