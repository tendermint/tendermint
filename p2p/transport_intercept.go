package p2p

import (
	"context"
	"net"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/service"
	"github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/p2p/interceptcontroller"
)

type InterceptedTransportOptions struct {
	mConnOptions MConnTransportOptions
}

// InterceptedTransport wraps around MConnTransport
type InterceptedTransport struct {
	service.BaseService
	mConnTransport *MConnTransport
	controller     interceptcontroller.Controller
}

// NewInterceptedTransport create InterceptedTransport
func NewInterceptedTransport(
	logger log.Logger,
	mConnConfig conn.MConnConfig,
	channelDescs []*ChannelDescriptor,
	options InterceptedTransportOptions,
) *InterceptedTransport {
	return &InterceptedTransport{
		mConnTransport: NewMConnTransport(
			logger,
			mConnConfig,
			channelDescs,
			options.mConnOptions,
		),
	}
}

func (t *InterceptedTransport) Protocols() []Protocol {
	return t.mConnTransport.Protocols()
}

func (t *InterceptedTransport) Endpoints() []Endpoint {
	return t.mConnTransport.Endpoints()
}

func (t *InterceptedTransport) Accept() (Connection, error) {
	// TODO: Need to wrap around mConnTransport's Accept.
	// Create channels that push to the controller and receive from the controller and use that to create the connection
	return nil, nil
}

func (t *InterceptedTransport) Dial(ctx context.Context, endpoint Endpoint) (Connection, error) {
	// TODO: Need to wrap around mConnTransport's Dial
	return nil, nil
}

func (t *InterceptedTransport) Listen(endpoint Endpoint) error {
	return t.mConnTransport.Listen(endpoint)
}

func (t *InterceptedTransport) Close() error {
	return t.mConnTransport.Close()
}

func (t *InterceptedTransport) OnStart() error {
	// TODO: Invoke go routines to poll the channels of the controller
	return nil
}

type InterceptedConnection struct {
	// TODO: Need to understand BaseService and implement the onStart method
	service.BaseService
	conn *mConnConnection
	// TODO: Figure out what needs to be sent in the channel and create struct for that
	inChan  chan interface{}
	outChan chan interface{}
}

func NewInterceptedConnection(
	logger log.Logger,
	conn net.Conn,
	mConnConfig conn.MConnConfig,
	channelDescs []*ChannelDescriptor,
	receiveChan chan interface{},
	sendChan chan interface{},
) *InterceptedConnection {
	return &InterceptedConnection{
		conn:    newMConnConnection(logger, conn, mConnConfig, channelDescs),
		inChan:  receiveChan,
		outChan: sendChan,
	}
}

func (c *InterceptedConnection) Handshake(
	ctx context.Context,
	nodeInfo NodeInfo,
	privKey crypto.PrivKey,
) (NodeInfo, crypto.PubKey, error) {
	// TODO: invoke Start when doing handshake
	return c.conn.Handshake(ctx, nodeInfo, privKey)
}

func (c *InterceptedConnection) OnStart() error {
	// TODO: Invoke go routines to poll the channels
	// refer service.BaseService
	return nil
}

func (c *InterceptedConnection) ReceiveMessage() (ChannelID, []byte, error) {
	// TODO: Need to use the inchan to receive message
	return 0, nil, nil
}

func (c *InterceptedConnection) SendMessage(_ ChannelID, _ []byte) error {
	// TODO: Need to use the outChan to send message
	return nil
}

func (c *InterceptedConnection) LocalEndpoint() Endpoint {
	return c.conn.LocalEndpoint()
}

func (c *InterceptedConnection) RemoteEndpoint() Endpoint {
	return c.conn.RemoteEndpoint()
}

func (c *InterceptedConnection) Close() error {
	// TODO: close the channels
	return nil
}
