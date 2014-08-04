package main

import (
	"container/heap"
	"fmt"
	"math/rand"
    "strings"
)

const seed = 0
const numNodes = 30000        // Total number of nodes to simulate
const minNumPeers = 7         // Each node should be connected to at least this many peers
const maxNumPeers = 10        // ... and at most this many
const latencyMS = int32(500)  // One way packet latency
const partTxMS = int32(100)   // Transmission time per peer of 4KB of data.
const sendQueueCapacity = 5  // Amount of messages to queue between peers.

func init() {
	rand.Seed(seed)
}

//-----------------------------------------------------------------------------

type Peer struct {
	node   *Node  // Pointer to node
	sent   int32  // Time of last packet send, including transmit time.
	remote int    // SomeNode.peers[x].node.peers[remote].node is SomeNode for all x.
	parts  []byte // [32]byte{} bitarray of received block pieces.
}

// Send a data event to the peer, or return false if queue is "full".
// Depending on how many event packets are "queued" for peer,
// the actual recvTime may be adjusted to be later.
func (p *Peer) sendEventData(event EventData) bool {
	desiredRecvTime := event.RecvTime()
	minRecvTime := p.sent + partTxMS + latencyMS
	if desiredRecvTime >= minRecvTime {
		p.node.sendEvent(event)
		p.sent += partTxMS
		return true
	} else {
		if (minRecvTime-desiredRecvTime)/partTxMS > sendQueueCapacity {
			return false
		} else {
			event.SetRecvTime(minRecvTime) // Adjust recvTime
			p.node.sendEvent(event)
			p.sent += partTxMS
			return true
		}
	}
}

// Returns true if the sendQueue is not "full"
func (p *Peer) canSendData(now int32) bool {
    return (p.sent - now) < sendQueueCapacity
}

// Since EventPart events are much smaller, we don't consider the transmit time,
// and assume that the sendQueue is always free.
func (p *Peer) sendEventParts(event EventParts) {
	p.node.sendEvent(event)
}

// Does the peer's .parts (as received by an EventParts event) contain part?
func (p *Peer) knownToHave(part uint8) bool {
	return p.parts[part/8]&(1<<(part%8)) > 0
}

//-----------------------------------------------------------------------------

type Node struct {
	index  int
	peers  []*Peer
	parts  []byte
	events *Heap
}

func (n *Node) sendEvent(event Event) {
	n.events.Push(event, event.RecvTime())
}

func (n *Node) recvEvent() Event {
	return n.events.Pop().(Event)
}

func (n *Node) receive(part uint8) bool {
	x := n.parts[part/8]
	nx := x | (1 << (part % 8))
	if x == nx {
		return false
	} else {
		n.parts[part/8] = nx
		return true
	}
}

// returns false if already connected, or remote node has too many connections.
func (n *Node) canConnectTo(node *Node) bool {
	if len(n.peers) > maxNumPeers {
		return false
	}
	for _, peer := range n.peers {
		if peer.node == node {
			return false
		}
	}
	return true
}

func (n *Node) isFull() bool {
	for _, part := range n.parts {
		if part != byte(0xff) {
			return false
		}
	}
	return true
}

func (n *Node) pickRandomForPeer(peer *Peer) (part uint8, ok bool) {
    peerParts := peer.parts
    nodeParts := n.parts
    randStart := rand.Intn(32)
    for i:=0; i<32; i++ {
        bytei := uint8((i+randStart) % 32)
        nByte := nodeParts[bytei]
        pByte := peerParts[bytei]
        iHas  := nByte & ^pByte
        if iHas > 0 {
            randBitStart := rand.Intn(8)
            //fmt.Println("//--")
            for j:=0; j<8; j++ {
                biti := uint8((j+randBitStart) % 8)
                //fmt.Printf("%X %v %v %v\n", iHas, j, biti, randBitStart)
                if (iHas & (1<<biti)) > 0 {
                    return 8*bytei + biti, true
                }
            }
            panic("should not happen")
        }
    }
    return 0, false
}

func (n *Node) debug() {
    lines := []string{}
    lines = append(lines, n.String())
    lines = append(lines, fmt.Sprintf("events: %v, parts: %X", n.events.Len(), n.parts))
    for _, p := range n.peers {
        part, ok := n.pickRandomForPeer(p)
        lines = append(lines, fmt.Sprintf("peer sent: %v, parts: %X, (%v/%v)", p.sent, p.parts, part, ok))
    }
    fmt.Println("//---------------")
    fmt.Println(strings.Join(lines, "\n"))
    fmt.Println("//---------------")
}

func (n *Node) String() string {
	return fmt.Sprintf("{N:%d}", n.index)
}

//-----------------------------------------------------------------------------

type Event interface {
	RecvTime() int32
	SetRecvTime(int32)
}

type EventData struct {
	time int32 // time of receipt.
    src  int   // src node's peer index on destination node
	part uint8
}

func (e EventData) RecvTime() int32 {
	return e.time
}

func (e EventData) SetRecvTime(time int32) {
	e.time = time
}

func (e EventData) String() string {
	return fmt.Sprintf("[%d:%d:%d]", e.time, e.src, e.part)
}

type EventParts struct {
	time  int32 // time of receipt.
	src   int   // src node's peer index on destination node.
	parts []byte
}

func (e EventParts) RecvTime() int32 {
	return e.time
}

func (e EventParts) SetRecvTime(time int32) {
	e.time = time
}

func (e EventParts) String() string {
	return fmt.Sprintf("[%d:%d:%d]", e.time, e.src, e.parts)
}

//-----------------------------------------------------------------------------

func createNetwork() []*Node {
	nodes := make([]*Node, numNodes)
	for i := 0; i < numNodes; i++ {
		n := &Node{
			index:  i,
			peers:  []*Peer{},
			parts:  make([]byte, 32),
			events: NewHeap(),
		}
		nodes[i] = n
	}
	for i := 0; i < numNodes; i++ {
		n := nodes[i]
		for j := 0; j < minNumPeers; j++ {
			if len(n.peers) > j {
				// Already set, continue
				continue
			}
			pidx := rand.Intn(numNodes)
			for !n.canConnectTo(nodes[pidx]) {
				pidx = rand.Intn(numNodes)
			}
			// connect to nodes[pidx]
			remote := nodes[pidx]
			remote_j := len(remote.peers)
			n.peers = append(n.peers, &Peer{node: remote, remote: remote_j, parts: make([]byte, 32)})
			remote.peers = append(remote.peers, &Peer{node: n, remote: j, parts: make([]byte, 32)})
		}
	}
	return nodes
}

func printNodes(nodes []*Node) {
	for _, node := range nodes {
		peerStr := ""
		for _, peer := range node.peers {
			peerStr += fmt.Sprintf(" %v", peer.node.index)
		}
		fmt.Printf("[%v] peers: %v\n", node.index, peerStr)
	}
}

func countFull(nodes []*Node) (fullCount int) {
	for _, node := range nodes {
		if node.isFull() {
			fullCount += 1
		}
	}
	return fullCount
}

func main() {

	// Global vars
	nodes := createNetwork()
	timeMS := int32(0)
	proposer := nodes[0]
	for i := 0; i < 32; i++ {
		proposer.parts[i] = byte(0xff)
	}
	//printNodes(nodes[:])

	// The proposer sends parts to all of its peers.
	for i := 0; i < len(proposer.peers); i++ {
		timeMS := int32(0) // scoped
		peer := proposer.peers[i]
		for j := 0; j < 256; j++ {
			// Send each part to a peer, but each peer starts at a different offset.
			part := uint8((j + i*25) % 256)
			recvTime := timeMS + latencyMS + partTxMS
			event := EventData{
				time: recvTime,
                src:  peer.remote,
				part: part,
			}
			peer.sendEventData(event)
			timeMS += partTxMS
		}
	}

	// Run simulation
	for {
		// Lets run the simulation for each user until endTimeMS
		// We use latencyMS/2 since causality has at least this much lag.
		endTimeMS := timeMS + latencyMS/2
		fmt.Printf("simulating until %v\n", endTimeMS)

		// Print out the network for debugging
		if true {
			for i := 0; i < 40; i++ {
				node := nodes[i]
				fmt.Printf("[%v] parts: %X\n", node.index, node.parts)
			}
		}

		for _, node := range nodes {

			// Iterate over the events of this node until event.time >= endTimeMS
			for {
				_event, ok := node.events.Peek().(Event)
				if !ok || _event.RecvTime() >= endTimeMS {
					break
				} else {
					node.events.Pop()
				}

				switch _event.(type) {
				case EventData:
					event := _event.(EventData)

					// Process this event
					if !node.receive(event.part) {
						// Already has this part, ignore this event.
						continue
					}

					// Let's iterate over peers & see which needs this piece.
					for _, peer := range node.peers {
						if !peer.knownToHave(event.part) {
                            peer.sendEventData(EventData{
                                time: event.time + latencyMS + partTxMS,
                                src:  peer.remote,
                                part: event.part,
                            })
                        } else {
                            continue
                        }
					}

				case EventParts:
					event := _event.(EventParts)
					node.peers[event.src].parts = event.parts
                    peer := node.peers[event.src]

                    // Lets blast the peer with random parts.
                    randomSent := 0
                    randomSentErr := 0
                    for peer.canSendData(event.time) {
                        part, ok := node.pickRandomForPeer(peer)
                        if ok {
                            randomSent += 1
                            sent := peer.sendEventData(EventData{
                                time: event.time + latencyMS + partTxMS,
                                src:  peer.remote,
                                part: part,
                            })
                            if !sent {
                                randomSentErr += 1
                            }
                        } else {
                            break
                        }
                    }
                    /*
                    if randomSent > 0 {
                        fmt.Printf("radom sent: %v %v", randomSent, randomSentErr)
                    }
                    */
				}

			}
		}

		// If network is full, quit.
		if countFull(nodes) == numNodes {
			fmt.Printf("Done! took %v ms", timeMS)
			break
		}

		// Lets increment the timeMS now
		timeMS += latencyMS / 2

        // Debug
        if timeMS >= 25000 {
            nodes[1].debug()
            for e := nodes[1].events.Pop(); e != nil; e = nodes[1].events.Pop() {
                fmt.Println(e)
            }
            return
        }

		// Send EventParts rather frequently. It's cheap.
		for _, node := range nodes {
			for _, peer := range node.peers {
				peer.sendEventParts(EventParts{
					time:  timeMS + latencyMS,
					src:   peer.remote,
					parts: node.parts,
				})
			}

			newParts := make([]byte, 32)
			copy(newParts, node.parts)
			node.parts = newParts
		}

	}
}

// ----------------------------------------------------------------------------

type Heap struct {
	pq priorityQueue
}

func NewHeap() *Heap {
	return &Heap{pq: make([]*pqItem, 0)}
}

func (h *Heap) Len() int {
	return len(h.pq)
}

func (h *Heap) Peek() interface{} {
	if len(h.pq) == 0 {
		return nil
	}
	return h.pq[0].value
}

func (h *Heap) Push(value interface{}, priority int32) {
	heap.Push(&h.pq, &pqItem{value: value, priority: priority})
}

func (h *Heap) Pop() interface{} {
	item := heap.Pop(&h.pq).(*pqItem)
	return item.value
}

/*
func main() {
    h := NewHeap()

    h.Push(String("msg1"), 1)
    h.Push(String("msg3"), 3)
    h.Push(String("msg2"), 2)

    fmt.Println(h.Pop())
    fmt.Println(h.Pop())
    fmt.Println(h.Pop())
}
*/

///////////////////////
// From: http://golang.org/pkg/container/heap/#example__priorityQueue

type pqItem struct {
	value    interface{}
	priority int32
	index    int
}

type priorityQueue []*pqItem

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	return pq[i].priority < pq[j].priority
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*pqItem)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

func (pq *priorityQueue) Update(item *pqItem, value interface{}, priority int32) {
	heap.Remove(pq, item.index)
	item.value = value
	item.priority = priority
	heap.Push(pq, item)
}
