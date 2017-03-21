package monitor

import (
	"encoding/json"
	"math"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/pkg/errors"
	crypto "github.com/tendermint/go-crypto"
	events "github.com/tendermint/go-events"
	rpc_client "github.com/tendermint/go-rpc/client"
	wire "github.com/tendermint/go-wire"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
	tmtypes "github.com/tendermint/tendermint/types"
	em "github.com/tendermint/tools/tm-monitor/eventmeter"
)

// remove when https://github.com/tendermint/go-rpc/issues/8 will be fixed
type rpcClientI interface {
	Call(method string, params map[string]interface{}, result interface{}) (interface{}, error)
}

const maxRestarts = 25

type Node struct {
	rpcAddr string

	IsValidator bool          `json:"is_validator"` // validator or non-validator?
	pubKey      crypto.PubKey `json:"pub_key"`

	Name         string  `json:"name"`
	Online       bool    `json:"online"`
	Height       uint64  `json:"height"`
	BlockLatency float64 `json:"block_latency" wire:"unsafe"` // ms, interval between block commits

	// em holds the ws connection. Each eventMeter callback is called in a separate go-routine.
	em eventMeter

	// rpcClient is an client for making RPC calls to TM
	rpcClient rpcClientI

	blockCh        chan<- tmtypes.Header
	blockLatencyCh chan<- float64
	disconnectCh   chan<- bool

	checkIsValidatorInterval time.Duration

	quit chan struct{}

	logger log.Logger
}

func NewNode(rpcAddr string, options ...func(*Node)) *Node {
	em := em.NewEventMeter(rpcAddr, UnmarshalEvent)
	rpcClient := rpc_client.NewClientURI(rpcAddr) // HTTP client by default
	return NewNodeWithEventMeterAndRpcClient(rpcAddr, em, rpcClient, options...)
}

func NewNodeWithEventMeterAndRpcClient(rpcAddr string, em eventMeter, rpcClient rpcClientI, options ...func(*Node)) *Node {
	n := &Node{
		rpcAddr:   rpcAddr,
		em:        em,
		rpcClient: rpcClient,
		Name:      rpcAddr,
		quit:      make(chan struct{}),
		checkIsValidatorInterval: 5 * time.Second,
		logger: log.NewNopLogger(),
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
	n.em.Subscribe(tmtypes.EventStringNewBlockHeader(), newBlockCallback(n))
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
	return func(metric *em.EventMetric, data events.EventData) {
		block := data.(tmtypes.EventDataNewBlockHeader).Header

		n.Height = uint64(block.Height)
		n.logger.Log("event", "new block", "height", block.Height, "numTxs", block.NumTxs)

		if n.blockCh != nil {
			n.blockCh <- *block
		}
	}
}

// implements eventmeter.EventLatencyFunc
func latencyCallback(n *Node) em.LatencyCallbackFunc {
	return func(latency float64) {
		n.BlockLatency = latency / 1000000.0 // ns to ms
		n.logger.Log("event", "new block latency", "latency", n.BlockLatency)

		if n.blockLatencyCh != nil {
			n.blockLatencyCh <- latency
		}
	}
}

// implements eventmeter.DisconnectCallbackFunc
func disconnectCallback(n *Node) em.DisconnectCallbackFunc {
	return func() {
		n.Online = false
		n.logger.Log("status", "down")

		if n.disconnectCh != nil {
			n.disconnectCh <- true
		}

		if err := n.RestartEventMeterBackoff(); err != nil {
			n.logger.Log("err", errors.Wrap(err, "restart failed"))
		} else {
			n.Online = true
			n.logger.Log("status", "online")

			if n.disconnectCh != nil {
				n.disconnectCh <- false
			}
		}
	}
}

func (n *Node) RestartEventMeterBackoff() error {
	attempt := 0

	for {
		d := time.Duration(math.Exp2(float64(attempt)))
		time.Sleep(d * time.Second)

		if err := n.em.Start(); err != nil {
			n.logger.Log("err", errors.Wrap(err, "restart failed"))
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

func (n *Node) NumValidators() (height uint64, num int, err error) {
	height, vals, err := n.validators()
	if err != nil {
		return 0, 0, err
	}
	return height, len(vals), nil
}

func (n *Node) validators() (height uint64, validators []*tmtypes.Validator, err error) {
	var result ctypes.TMResult
	if _, err = n.rpcClient.Call("validators", nil, &result); err != nil {
		return 0, make([]*tmtypes.Validator, 0), err
	}
	vals := result.(*ctypes.ResultValidators)
	return uint64(vals.BlockHeight), vals.Validators, nil
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
			key, err := n.getPubKey()
			if err == nil && v.PubKey == key {
				n.IsValidator = true
			}
		}
	} else {
		n.logger.Log("err", errors.Wrap(err, "check is validator failed"))
	}
}

func (n *Node) getPubKey() (crypto.PubKey, error) {
	if n.pubKey != nil {
		return n.pubKey, nil
	}

	var result ctypes.TMResult
	_, err := n.rpcClient.Call("status", nil, &result)
	if err != nil {
		return nil, err
	}
	status := result.(*ctypes.ResultStatus)
	n.pubKey = status.PubKey
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
	var err error
	result := new(ctypes.TMResult)
	wire.ReadJSONPtr(result, b, &err)
	if err != nil {
		return "", nil, err
	}
	event, ok := (*result).(*ctypes.ResultEvent)
	if !ok {
		return "", nil, nil // TODO: handle non-event messages (ie. return from subscribe/unsubscribe)
		// fmt.Errorf("Result is not type *ctypes.ResultEvent. Got %v", reflect.TypeOf(*result))
	}
	return event.Name, event.Data, nil
}
