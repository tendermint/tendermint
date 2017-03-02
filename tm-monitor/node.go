package main

import (
	"encoding/json"
	"fmt"
	"math"
	"time"

	em "github.com/tendermint/go-event-meter"
	events "github.com/tendermint/go-events"
	rpc_client "github.com/tendermint/go-rpc/client"
	tmtypes "github.com/tendermint/tendermint/types"

	crypto "github.com/tendermint/go-crypto"
	wire "github.com/tendermint/go-wire"
	ctypes "github.com/tendermint/tendermint/rpc/core/types"
)

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

	// rpcClient is an http client for making RPC calls to TM
	rpcClient *rpc_client.ClientURI

	blockCh        chan<- tmtypes.Header
	blockLatencyCh chan<- float64
	disconnectCh   chan<- bool

	quit chan struct{}
}

func NewNode(rpcAddr string) *Node {
	em := em.NewEventMeter(rpcAddr, UnmarshalEvent)
	return NewNodeWithEventMeter(rpcAddr, em)
}

func NewNodeWithEventMeter(rpcAddr string, em eventMeter) *Node {
	return &Node{
		rpcAddr:   rpcAddr,
		em:        em,
		rpcClient: rpc_client.NewClientURI(rpcAddr),
		Name:      rpcAddr,
		quit:      make(chan struct{}),
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

func (n *Node) Start() error {
	if _, err := n.em.Start(); err != nil {
		return err
	}

	n.em.RegisterLatencyCallback(latencyCallback(n))
	n.em.Subscribe(tmtypes.EventStringNewBlockHeader(), newBlockCallback(n))
	n.em.RegisterDisconnectCallback(disconnectCallback(n))

	n.Online = true

	go n.checkIsValidator()

	return nil
}

func (n *Node) Stop() {
	n.Online = false

	n.em.RegisterLatencyCallback(nil)
	n.em.Unsubscribe(tmtypes.EventStringNewBlockHeader())
	n.em.RegisterDisconnectCallback(nil)

	// FIXME stop blocks at event_meter.go:140
	// n.em.Stop()

	close(n.quit)
}

// implements eventmeter.EventCallbackFunc
func newBlockCallback(n *Node) em.EventCallbackFunc {
	return func(metric *em.EventMetric, data events.EventData) {
		block := data.(tmtypes.EventDataNewBlockHeader).Header

		n.Height = uint64(block.Height)

		if n.blockCh != nil {
			n.blockCh <- *block
		}
	}
}

// implements eventmeter.EventLatencyFunc
func latencyCallback(n *Node) em.LatencyCallbackFunc {
	return func(latency float64) {
		n.BlockLatency = latency / 1000000.0 // ns to ms
		if n.blockLatencyCh != nil {
			n.blockLatencyCh <- latency
		}
	}
}

// implements eventmeter.DisconnectCallbackFunc
func disconnectCallback(n *Node) em.DisconnectCallbackFunc {
	return func() {
		n.Online = false
		if n.disconnectCh != nil {
			n.disconnectCh <- true
		}

		if err := n.RestartBackOff(); err != nil {
			log.Error(err.Error())
		} else {
			n.Online = true
			if n.disconnectCh != nil {
				n.disconnectCh <- false
			}
		}
	}
}

func (n *Node) RestartBackOff() error {
	attempt := 0

	for {
		d := time.Duration(math.Exp2(float64(attempt)))
		time.Sleep(d * time.Second)

		if err := n.Start(); err != nil {
			log.Debug("Can't connect to node %v due to %v", n, err)
		} else {
			// TODO: authenticate pubkey
			return nil
		}

		attempt++

		if attempt > maxRestarts {
			return fmt.Errorf("Reached max restarts for node %v", n)
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

func (n *Node) checkIsValidator() {
	for {
		select {
		case <-n.quit:
			return
		case <-time.After(5 * time.Second):
			_, validators, err := n.validators()
			if err == nil {
				for _, v := range validators {
					key, err := n.getPubKey()
					if err == nil && v.PubKey == key {
						n.IsValidator = true
					}
				}
			} else {
				log.Debug(err.Error())
			}
		}
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
	Start() (bool, error)
	Stop() bool
	RegisterLatencyCallback(em.LatencyCallbackFunc)
	RegisterDisconnectCallback(em.DisconnectCallbackFunc)
	Subscribe(string, em.EventCallbackFunc) error
	Unsubscribe(string) error
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
