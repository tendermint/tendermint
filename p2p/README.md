# P2P Module

P2P provides an abstraction around peer-to-peer communication.<br/>
Communication happens via Agents that react to messages from peers.<br/>
Each Agent has one or more Channels of communication for each Peer.<br/>
Channels are multiplexed automatically and can be configured.<br/>
A Switch is started upon app start, and handles Peer management.<br/>
A PEXAgent implementation is provided to automate peer discovery.<br/>

## Usage

MempoolAgent started from the following template code.<br/>
Modify the snippet below according to your needs.<br/>
Check out the ConsensusAgent for an example of tracking peer state.<br/>

```golang
package mempool

import (
	"bytes"
	"fmt"
	"io"
	"sync/atomic"

	. "github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/blocks"
	"github.com/tendermint/tendermint/p2p"
)

var (
	MempoolCh = byte(0x30)
)

// MempoolAgent handles mempool tx broadcasting amongst peers.
type MempoolAgent struct {
	sw       *p2p.Switch
	swEvents chan interface{}
	quit     chan struct{}
	started  uint32
	stopped  uint32
}

func NewMempoolAgent(sw *p2p.Switch) *MempoolAgent {
	swEvents := make(chan interface{})
	sw.AddEventListener("MempoolAgent.swEvents", swEvents)
	memA := &MempoolAgent{
		sw:       sw,
		swEvents: swEvents,
		quit:     make(chan struct{}),
	}
	return memA
}

func (memA *MempoolAgent) Start() {
	if atomic.CompareAndSwapUint32(&memA.started, 0, 1) {
		log.Info("Starting MempoolAgent")
		go memA.switchEventsRoutine()
		go memA.gossipTxRoutine()
	}
}

func (memA *MempoolAgent) Stop() {
	if atomic.CompareAndSwapUint32(&memA.stopped, 0, 1) {
		log.Info("Stopping MempoolAgent")
		close(memA.quit)
		close(memA.swEvents)
	}
}

// Handle peer new/done events
func (memA *MempoolAgent) switchEventsRoutine() {
	for {
		swEvent, ok := <-memA.swEvents
		if !ok {
			break
		}
		switch swEvent.(type) {
		case p2p.SwitchEventNewPeer:
			// event := swEvent.(p2p.SwitchEventNewPeer)
			// NOTE: set up peer state
		case p2p.SwitchEventDonePeer:
			// event := swEvent.(p2p.SwitchEventDonePeer)
			// NOTE: tear down peer state
		default:
			log.Warning("Unhandled switch event type")
		}
	}
}

func (memA *MempoolAgent) gossipTxRoutine() {
OUTER_LOOP:
	for {
		// Receive incoming message on MempoolCh
		inMsg, ok := memA.sw.Receive(MempoolCh)
		if !ok {
			break OUTER_LOOP // Client has stopped
		}
		_, msg_ := decodeMessage(inMsg.Bytes)
		log.Info("gossipMempoolRoutine received %v", msg_)

		switch msg_.(type) {
		case *TxMessage:
			// msg := msg_.(*TxMessage)
			// handle msg

		default:
			// Ignore unknown message
			// memA.sw.StopPeerForError(inMsg.MConn.Peer, errInvalidMessage)
		}
	}

	// Cleanup
}

```


## Channels

Each peer connection is multiplexed into channels.
The p2p module comes with a channel implementation used for peer
discovery (called PEX, short for "peer exchange").

<table>
  <tr>
    <td><b>Channel</b></td>
    <td>"PEX"</td>
  </tr>
  <tr>
    <td><b>Messages</b></td>
    <td>
      <ul>
        <li>pexRequestMsg</li>
        <li>pexResponseMsg</li>
      </ul>
    </td>
  </tr>
</table>
<hr />

## Resources

* http://www.upnp-hacks.org/upnp.html
