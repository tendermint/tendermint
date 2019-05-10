package monitor

import (
	"encoding/json"
	"math"
	"time"

	"github.com/pkg/errors"

	"github.com/tendermint/tendermint/crypto"
	"github.com/tendermint/tendermint/libs/events"
	"github.com/tendermint/tendermint/libs/log"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpc_client "github.com/tendermint/tendermint/rpc/lib/client"
	em "github.com/tendermint/tendermint/tools/tm-monitor/eventmeter"
	tmtypes "github.com/tendermint/tendermint/types"
)

const maxRestarts = 25

type Node struct {
	rpcAddr string

	IsValidator bool `json:"is_validator"` // validator or non-validator?
	pubKey      crypto.PubKey

	Name         string  `json:"name"`
	Online       bool    `json:"online"`
	Height       int64   `json:"height"`
	BlockLatency float64 `json:"block_latency" amino:"unsafe"` // ms, interval between block commits

	// em holds the ws connection. Each eventMeter callback is called in a separate go-routine.
	em eventMeter

	// rpcClient is an client for making RPC calls to TM
	rpcClient rpc_client.HTTPClient

	blockCh        chan<- tmtypes.Header
	blockLatencyCh chan<- float64
	disconnectCh   chan<- bool

	checkIsValidatorInterval time.Duration

	quit chan struct{}

	logger log.Logger
}

func NewNode(rpcAddr string, options ...func(*Node)) *Node {
	em := em.NewEventMeter(rpcAddr, UnmarshalEvent)
	rpcClient := rpc_client.NewURIClient(rpcAddr) // HTTP client by default
	rpcClient.SetCodec(cdc)
	return NewNodeWithEventMeterAndRpcClient(rpcAddr, em, rpcClient, options...)
}

func NewNodeWithEventMeterAndRpcClient(rpcAddr string, em eventMeter, rpcClient rpc_client.HTTPClient, options ...func(*Node)) *Node {
	n := &Node{
		rpcAddr:                  rpcAddr,
		em:                       em,
		rpcClient:                rpcClient,
		Name:                     rpcAddr,
		quit:                     make(chan struct{}),
		checkIsValidatorInterval: 5 * time.Second,
		logger:                   log.NewNopLogger(),
	}

	for _, option := range options {
		option(n)
	}

	return n
}

// SetCheckIsValidatorInterval lets you change interval for checking whenever
// node is still a validator or not.
func SetCheckIsValidatorInterval(d time.Duration) func(n *Node) {
	return func(n *Node) {
		n.checkIsValidatorInterval = d
	}
}

func (n *Node) SendBlocksTo(ch chan<- tmtypes.Header) {
	n.blockCh = ch
}

func (n *Node) SendBlockLatenciesTo(ch chan<- float64) {
	n.blockLatencyCh = ch
}

func (n *Node) NotifyAboutDisconnects(ch chan<- bool) {
	n.disconnectCh = ch
}

// SetLogger lets you set your own logger
func (n *Node) SetLogger(l log.Logger) {
	n.logger = l
	n.em.SetLogger(l)
}

func (n *Node) Start() error {
	if err := n.em.Start(); err != nil {
		return err
	}

	n.em.RegisterLatencyCallback(latencyCallback(n))
	err := n.em.Subscribe(tmtypes.EventQueryNewBlockHeader.String(), newBlockCallback(n))
	if err != nil {
		return err
	}
	n.em.RegisterDisconnectCallback(disconnectCallback(n))

	n.Online = true

	n.checkIsValidator()
	go n.checkIsValidatorLoop()

	return nil
}

func (n *Node) Stop() {
	n.Online = false

	n.em.Stop()

	close(n.quit)
}

// implements eventmeter.EventCallbackFunc
func newBlockCallback(n *Node) em.EventCallbackFunc {
	return func(metric *em.EventMetric, data interface{}) {
		block := data.(tmtypes.TMEventData).(tmtypes.EventDataNewBlockHeader).Header

		n.Height = block.Height
		n.logger.Info("new block", "height", block.Height, "numTxs", block.NumTxs)

		if n.blockCh != nil {
			n.blockCh <- block
		}
	}
}

// implements eventmeter.EventLatencyFunc
func latencyCallback(n *Node) em.LatencyCallbackFunc {
	return func(latency float64) {
		n.BlockLatency = latency / 1000000.0 // ns to ms
		n.logger.Info("new block latency", "latency", n.BlockLatency)

		if n.blockLatencyCh != nil {
			n.blockLatencyCh <- latency
		}
	}
}

// implements eventmeter.DisconnectCallbackFunc
func disconnectCallback(n *Node) em.DisconnectCallbackFunc {
	return func() {
		n.Online = false
		n.logger.Info("status", "down")

		if n.disconnectCh != nil {
			n.disconnectCh <- true
		}
	}
}

func (n *Node) RestartEventMeterBackoff() error {
	attempt := 0

	for {
		d := time.Duration(math.Exp2(float64(attempt)))
		time.Sleep(d * time.Second)

		if err := n.em.Start(); err != nil {
			n.logger.Info("restart failed", "err", err)
		} else {
			// TODO: authenticate pubkey
			return nil
		}

		attempt++

		if attempt > maxRestarts {
			return errors.New("Reached max restarts")
		}
	}
}

func (n *Node) NumValidators() (height int64, num int, err error) {
	height, vals, err := n.validators()
	if err != nil {
		return 0, 0, err
	}
	return height, len(vals), nil
}

func (n *Node) validators() (height int64, validators []*tmtypes.Validator, err error) {
	vals := new(ctypes.ResultValidators)
	if _, err = n.rpcClient.Call("validators", nil, vals); err != nil {
		return 0, make([]*tmtypes.Validator, 0), err
	}
	return vals.BlockHeight, vals.Validators, nil
}

func (n *Node) checkIsValidatorLoop() {
	for {
		select {
		case <-n.quit:
			return
		case <-time.After(n.checkIsValidatorInterval):
			n.checkIsValidator()
		}
	}
}

func (n *Node) checkIsValidator() {
	_, validators, err := n.validators()
	if err == nil {
		for _, v := range validators {
			key, err1 := n.getPubKey()
			if err1 == nil && v.PubKey.Equals(key) {
				n.IsValidator = true
			}
		}
	} else {
		n.logger.Info("check is validator failed", "err", err)
	}
}

func (n *Node) getPubKey() (crypto.PubKey, error) {
	if n.pubKey != nil {
		return n.pubKey, nil
	}

	status := new(ctypes.ResultStatus)
	_, err := n.rpcClient.Call("status", nil, status)
	if err != nil {
		return nil, err
	}
	n.pubKey = status.ValidatorInfo.PubKey
	return n.pubKey, nil
}

type eventMeter interface {
	Start() error
	Stop()
	RegisterLatencyCallback(em.LatencyCallbackFunc)
	RegisterDisconnectCallback(em.DisconnectCallbackFunc)
	Subscribe(string, em.EventCallbackFunc) error
	Unsubscribe(string) error
	SetLogger(l log.Logger)
}

// UnmarshalEvent unmarshals a json event
func UnmarshalEvent(b json.RawMessage) (string, events.EventData, error) {
	event := new(ctypes.ResultEvent)
	if err := cdc.UnmarshalJSON(b, event); err != nil {
		return "", nil, err
	}
	return event.Query, event.Data, nil
}
