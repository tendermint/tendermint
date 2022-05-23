package p2p

import (
	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/tendermint/tendermint/config"
)

func NewHost(conf *config.P2PConfig) (host.Host, error)        { return nil, nil }
func NewPubSub(conf *config.P2PConfig) (*pubsub.PubSub, error) { return nil, nil }
