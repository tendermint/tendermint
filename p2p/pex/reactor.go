package pex

import (
	"context"
	"fmt"
	"time"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/p2p"
	protop2p "github.com/tendermint/tendermint/proto/tendermint/p2p"
)

var (
	_ service.Service = (*ReactorV2)(nil)
	_ p2p.Wrapper     = (*protop2p.PexMessage)(nil)
)

const (
	maxAddresses   uint16 = 100
	resolveTimeout        = 3 * time.Second
)

// ReactorV2 is a PEX reactor for the new P2P stack. The legacy reactor
// is Reactor.
//
// FIXME: Rename this when Reactor is removed, and consider moving to p2p/.
type ReactorV2 struct {
	service.BaseService

	peerManager *p2p.PeerManager
	pexCh       *p2p.Channel
	peerUpdates *p2p.PeerUpdates
	closeCh     chan struct{}
}

// NewReactor returns a reference to a new reactor.
func NewReactorV2(
	logger log.Logger,
	peerManager *p2p.PeerManager,
	pexCh *p2p.Channel,
	peerUpdates *p2p.PeerUpdates,
) *ReactorV2 {
	r := &ReactorV2{
		peerManager: peerManager,
		pexCh:       pexCh,
		peerUpdates: peerUpdates,
		closeCh:     make(chan struct{}),
	}

	r.BaseService = *service.NewBaseService(logger, "PEX", r)
	return r
}

// OnStart starts separate go routines for each p2p Channel and listens for
// envelopes on each. In addition, it also listens for peer updates and handles
// messages on that p2p channel accordingly. The caller must be sure to execute
// OnStop to ensure the outbound p2p Channels are closed.
func (r *ReactorV2) OnStart() error {
	go r.processPexCh()
	go r.processPeerUpdates()
	return nil
}

// OnStop stops the reactor by signaling to all spawned goroutines to exit and
// blocking until they all exit.
func (r *ReactorV2) OnStop() {
	// Close closeCh to signal to all spawned goroutines to gracefully exit. All
	// p2p Channels should execute Close().
	close(r.closeCh)

	// Wait for all p2p Channels to be closed before returning. This ensures we
	// can easily reason about synchronization of all p2p Channels and ensure no
	// panics will occur.
	<-r.pexCh.Done()
	<-r.peerUpdates.Done()
}

// handlePexMessage handles envelopes sent from peers on the PexChannel.
func (r *ReactorV2) handlePexMessage(envelope p2p.Envelope) error {
	logger := r.Logger.With("peer", envelope.From)

	// FIXME: We may want to add DoS protection here, by rate limiting peers and
	// only processing addresses we actually requested.
	switch msg := envelope.Message.(type) {
	case *protop2p.PexRequest:
		pexAddresses := r.resolve(r.peerManager.Advertise(envelope.From, maxAddresses), maxAddresses)
		r.pexCh.Out <- p2p.Envelope{
			To:      envelope.From,
			Message: &protop2p.PexResponse{Addresses: pexAddresses},
		}

	case *protop2p.PexResponse:
		for _, pexAddress := range msg.Addresses {
			peerAddress, err := p2p.ParseNodeAddress(
				fmt.Sprintf("%s@%s:%d", pexAddress.ID, pexAddress.IP, pexAddress.Port))
			if err != nil {
				logger.Debug("invalid PEX address", "address", pexAddress, "err", err)
				continue
			}
			if err = r.peerManager.Add(peerAddress); err != nil {
				logger.Debug("failed to register PEX address", "address", peerAddress, "err", err)
			}
		}

	default:
		return fmt.Errorf("received unknown message: %T", msg)
	}

	return nil
}

// resolve resolves a set of peer addresses into PEX addresses.
//
// FIXME: This is necessary because the current PEX protocol only supports
// IP/port pairs, while the P2P stack uses NodeAddress URLs. The PEX protocol
// should really use URLs too, to exchange DNS names instead of IPs and allow
// different transport protocols (e.g. QUIC and MemoryTransport).
//
// FIXME: We may want to cache and parallelize this, but for now we'll just rely
// on the operating system to cache it for us.
func (r *ReactorV2) resolve(addresses []p2p.NodeAddress, limit uint16) []protop2p.PexAddress {
	pexAddresses := make([]protop2p.PexAddress, 0, len(addresses))
	for _, address := range addresses {
		ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
		endpoints, err := address.Resolve(ctx)
		cancel()
		if err != nil {
			r.Logger.Debug("failed to resolve address", "address", address, "err", err)
			continue
		}
		for _, endpoint := range endpoints {
			if len(pexAddresses) >= int(limit) {
				return pexAddresses

			} else if endpoint.IP != nil {
				// PEX currently only supports IP-networked transports (as
				// opposed to e.g. p2p.MemoryTransport).
				pexAddresses = append(pexAddresses, protop2p.PexAddress{
					ID:   string(address.NodeID),
					IP:   endpoint.IP.String(),
					Port: uint32(endpoint.Port),
				})
			}
		}
	}
	return pexAddresses
}

// handleMessage handles an Envelope sent from a peer on a specific p2p Channel.
// It will handle errors and any possible panics gracefully. A caller can handle
// any error returned by sending a PeerError on the respective channel.
func (r *ReactorV2) handleMessage(chID p2p.ChannelID, envelope p2p.Envelope) (err error) {
	defer func() {
		if e := recover(); e != nil {
			err = fmt.Errorf("panic in processing message: %v", e)
		}
	}()

	r.Logger.Debug("received message", "peer", envelope.From)

	switch chID {
	case p2p.ChannelID(PexChannel):
		err = r.handlePexMessage(envelope)

	default:
		err = fmt.Errorf("unknown channel ID (%d) for envelope (%v)", chID, envelope)
	}

	return err
}

// processPexCh implements a blocking event loop where we listen for p2p
// Envelope messages from the pexCh.
func (r *ReactorV2) processPexCh() {
	defer r.pexCh.Close()

	for {
		select {
		case envelope := <-r.pexCh.In:
			if err := r.handleMessage(r.pexCh.ID, envelope); err != nil {
				r.Logger.Error("failed to process message", "ch_id", r.pexCh.ID, "envelope", envelope, "err", err)
				r.pexCh.Error <- p2p.PeerError{
					NodeID: envelope.From,
					Err:    err,
				}
			}

		case <-r.closeCh:
			r.Logger.Debug("stopped listening on PEX channel; closing...")
			return
		}
	}
}

// processPeerUpdate processes a PeerUpdate. For added peers, PeerStatusUp, we
// send a request for addresses.
func (r *ReactorV2) processPeerUpdate(peerUpdate p2p.PeerUpdate) {
	r.Logger.Debug("received peer update", "peer", peerUpdate.NodeID, "status", peerUpdate.Status)

	if peerUpdate.Status == p2p.PeerStatusUp {
		r.pexCh.Out <- p2p.Envelope{
			To:      peerUpdate.NodeID,
			Message: &protop2p.PexRequest{},
		}
	}
}

// processPeerUpdates initiates a blocking process where we listen for and handle
// PeerUpdate messages. When the reactor is stopped, we will catch the signal and
// close the p2p PeerUpdatesCh gracefully.
func (r *ReactorV2) processPeerUpdates() {
	defer r.peerUpdates.Close()

	for {
		select {
		case peerUpdate := <-r.peerUpdates.Updates():
			r.processPeerUpdate(peerUpdate)

		case <-r.closeCh:
			r.Logger.Debug("stopped listening on peer updates channel; closing...")
			return
		}
	}
}
