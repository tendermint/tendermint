package p2p

import (
	"context"
	"errors"

	"github.com/libp2p/go-libp2p-core/host"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/tendermint/tendermint/config"
)

// NewHost constructs a default networking connection for a libp2p
// network and returns the top level host object.
func NewHost(conf *config.P2PConfig) (host.Host, error) { return nil, errors.New("not implemented") }

// NewPubSub constructs a pubsub protocol using a libp2p host object.
func NewPubSub(ctx context.Context, conf *config.P2PConfig, host host.Host) (*pubsub.PubSub, error) {
	return nil, errors.New("not implemented")
}
