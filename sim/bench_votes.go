package main

import (
	"bufio"
	"container/heap"
	"fmt"
	"math/rand"
	"os"
)

const seed = 0
const numNodes = 6400 // Total number of nodes to simulate
const numNodes8 = (numNodes + 7) / 8
const minNumPeers = 8        // Each node should be connected to at least this many peers
const maxNumPeers = 12       // ... and at most this many
const latencyMS = int32(500) // One way packet latency
const partTxMS = int32(3)    // Transmission time per peer of 100B of data.
const sendQueueCapacity = 3200 // Amount of messages to queue between peers.
const maxAllowableRank = 2   // After this, the data is considered waste.
const tryUnsolicited = 0.02   // Chance of sending an unsolicited piece of data.

var log *bufio.Writer

func init() {
	rand.Seed(seed)
	openFile()
}

//-----------------------------------------------------------------------------

func openFile() {
	// open output file
	fo, err := os.Create("output.txt")
	if err != nil {
		panic(err)
	}
	// make a write buffer
	log = bufio.NewWriter(fo)
}

func logWrite(s string) {
	log.Write([]byte(s))
}

//-----------------------------------------------------------------------------

type Peer struct {
	node   *Node  // Pointer to node
	sent   int32  // Time of last packet send, including transmit time.
	remote int    // SomeNode.peers[x].node.peers[remote].node is SomeNode for all x.
	wanted []byte // Bitarray of wanted pieces.
	given  []byte // Bitarray of given pieces.
}

func newPeer(pNode *Node, remote int) *Peer {
	peer := &Peer{
		node:   pNode,
		remote: remote,
		wanted: make([]byte, numNodes8),
		given:  make([]byte, numNodes8),
	}
	for i := 0; i < numNodes8; i++ {
		peer.wanted[i] = byte(0xff)
	}
	return peer
}

// Send a data event to the peer, or return false if queue is "full".
// Depending on how many event packets are "queued" for peer,
// the actual recvTime may be adjusted to be later.
func (p *Peer) sendEventData(event EventData) bool {
	desiredRecvTime := event.RecvTime()
	minRecvTime := p.sent + partTxMS + latencyMS
	if desiredRecvTime >= minRecvTime {
		p.node.sendEvent(event)
		// p.sent + latencyMS == desiredRecvTime
		// when desiredRecvTime == minRecvTime,
		// p.sent += partTxMS
		p.sent = desiredRecvTime - latencyMS
		return true
	} else {
		if (minRecvTime-desiredRecvTime)/partTxMS > sendQueueCapacity {
			return false
		} else {
			event.time = minRecvTime // Adjust recvTime
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
func (p *Peer) sendEventDataResponse(event EventDataResponse) {
	p.node.sendEvent(event)
}

// Does the peer's .wanted (as received by an EventDataResponse event) contain part?
func (p *Peer) wants(part uint16) bool {
	return p.wanted[part/8]&(1<<(part%8)) > 0
}

func (p *Peer) setWants(part uint16, want bool) {
	if want {
		p.wanted[part/8] |= (1 << (part % 8))
	} else {
		p.wanted[part/8] &= ^(1 << (part % 8))
	}
}

func (p *Peer) setGiven(part uint16) {
	p.given[part/8] |= (1 << (part % 8))
}

// Reset state in preparation for new "round"
func (p *Peer) reset() {
	for i := 0; i < numNodes8; i++ {
		p.given[i] = byte(0x00)
	}
	p.sent = 0
}

//-----------------------------------------------------------------------------

type Node struct {
	index      int
	peers      []*Peer
	parts      []byte  // Bitarray of received parts.
	partsCount []uint8 // Count of how many times parts were received.
	events     *Heap
}

// Reset state in preparation for new "round"
func (n *Node) reset() {
	for i := 0; i < numNodes8; i++ {
		n.parts[i] = byte(0x00)
	}
	for i := 0; i < numNodes; i++ {
		n.partsCount[i] = uint8(0)
	}
	n.events = NewHeap()
	for _, peer := range n.peers {
		peer.reset()
	}
}

func (n *Node) fill() float64 {
	gotten := 0
	for _, count := range n.partsCount {
		if count > 0 {
			gotten += 1
		}
	}
	return float64(gotten) / float64(numNodes)
}

func (n *Node) sendEvent(event Event) {
	n.events.Push(event, event.RecvTime())
}

func (n *Node) recvEvent() Event {
	return n.events.Pop().(Event)
}

func (n *Node) receive(part uint16) uint8 {
	/*
		defer func() {
			e := recover()
			if e != nil {
				fmt.Println(part, len(n.parts), len(n.partsCount), part/8)
				panic(e)
			}
		}()
	*/
	n.parts[part/8] |= (1 << (part % 8))
	n.partsCount[part] += 1
	return n.partsCount[part]
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
	for _, count := range n.partsCount {
		if count == 0 {
			return false
		}
	}
	return true
}

func (n *Node) String() string {
	return fmt.Sprintf("{N:%d}", n.index)
}

//-----------------------------------------------------------------------------

type Event interface {
	RecvTime() int32
}

type EventData struct {
	time int32 // time of receipt.
	src  int   // src node's peer index on destination node
	part uint16
}

func (e EventData) RecvTime() int32 {
	return e.time
}

func (e EventData) String() string {
	return fmt.Sprintf("[%d:%d:%d]", e.time, e.src, e.part)
}

type EventDataResponse struct {
	time int32  // time of receipt.
	src  int    // src node's peer index on destination node.
	part uint16 // in response to given part
	rank uint8  // if this is 1, node was first to give peer part.
}

func (e EventDataResponse) RecvTime() int32 {
	return e.time
}

func (e EventDataResponse) String() string {
	return fmt.Sprintf("[%d:%d:%d:%d]", e.time, e.src, e.part, e.rank)
}

//-----------------------------------------------------------------------------

func createNetwork() []*Node {
	nodes := make([]*Node, numNodes)
	for i := 0; i < numNodes; i++ {
		n := &Node{
			index:      i,
			peers:      []*Peer{},
			parts:      make([]byte, numNodes8),
			partsCount: make([]uint8, numNodes),
			events:     NewHeap(),
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
			n.peers = append(n.peers, newPeer(remote, remote_j))
			remote.peers = append(remote.peers, newPeer(n, j))
		}
	}
	return nodes
}

func countFull(nodes []*Node) (fullCount int) {
	for _, node := range nodes {
		if node.isFull() {
			fullCount += 1
		}
	}
	return fullCount
}

type runStat struct {
	time int32   // time for all events to propagate
	fill float64 // avg % of pieces gotten
	succ float64 // % of times the sendQueue was not full
	dups float64 // % of times that a received data was duplicate
}

func (s runStat) String() string {
	return fmt.Sprintf("{t:%v/fi:%.5f/su:%.5f/du:%.5f}", s.time, s.fill, s.succ, s.dups)
}

func main() {

	// Global vars
	nodes := createNetwork()
	runStats := []runStat{}

	// Keep iterating and improving .wanted
	for {
		timeMS := int32(0)

		// Each node sends a part to its peers.
		for _, node := range nodes {
			// reset all node state.
			node.reset()
		}

		// Each node sends a part to its peers.
		for i, node := range nodes {
			// TODO: make it staggered.
			timeMS := int32(0) // scoped
			for _, peer := range node.peers {
				recvTime := timeMS + latencyMS + partTxMS
				event := EventData{
					time: recvTime,
					src:  peer.remote,
					part: uint16(i),
				}
				peer.sendEventData(event)
				//timeMS += partTxMS
			}
		}

		numEventsZero := 0  // times no events have occured
		numSendSuccess := 0 // times data send was successful
		numSendFailure := 0 // times data send failed due to queue being full
		numReceives := 0    // number of data items received
		numDups := 0        // number of data items that were duplicate

		// Run simulation
		for {
			// Lets run the simulation for each user until endTimeMS
			// We use latencyMS/2 since causality has at least this much lag.
			endTimeMS := timeMS + latencyMS/2

			// Print out the network for debugging
			/*
				fmt.Printf("simulating until %v\n", endTimeMS)
				if true {
					for i := 0; i < 40; i++ {
						node := nodes[i]
						fmt.Printf("[%v] parts: %X\n", node.index, node.parts)
					}
				}
			*/

			numEvents := 0
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
						numEvents++

						// Process this event
						rank := node.receive(event.part)
						// Send rank back to peer
						// NOTE: in reality, maybe this doesn't always happen.
						srcPeer := node.peers[event.src]
						srcPeer.setGiven(event.part) // HACK
						srcPeer.sendEventDataResponse(EventDataResponse{
							time: event.time + latencyMS, // TODO: responseTxMS ?
							src:  srcPeer.remote,
							part: event.part,
							rank: rank,
						})

						//logWrite(fmt.Sprintf("[%v] t:%v s:%v -> n:%v p:%v r:%v\n", len(runStats), event.time, srcPeer.node.index, node.index, event.part, rank))

						if rank > 1 {
							// Already has this part, ignore this event.
							numReceives++
							numDups++
							continue
						} else {
							numReceives++
						}

						// Let's iterate over peers & see which wants this piece.
						// We don't need to check peer.given because duplicate parts are ignored.
						for _, peer := range node.peers {
							if peer.wants(event.part) {
								//fmt.Print("w")
								sent := peer.sendEventData(EventData{
									time: event.time + latencyMS + partTxMS,
									src:  peer.remote,
									part: event.part,
								})
								if sent {
									//logWrite(fmt.Sprintf("[%v] t:%v S:%v n:%v -> p:%v %v WS\n", len(runStats), event.time, srcPeer.node.index, node.index, peer.node.index, event.part))
									peer.setGiven(event.part)
									numSendSuccess++
								} else {
									//logWrite(fmt.Sprintf("[%v] t:%v S:%v n:%v -> p:%v %v WF\n", len(runStats), event.time, srcPeer.node.index, node.index, peer.node.index, event.part))
									numSendFailure++
								}
							} else {
								//fmt.Print("!")
								// Peer doesn't want it, but sporadically we'll try sending it anyways.
								/*
								if rand.Float32() < tryUnsolicited {
									sent := peer.sendEventData(EventData{
										time: event.time + latencyMS + partTxMS,
										src:  peer.remote,
										part: event.part,
									})
									if sent {
										//logWrite(fmt.Sprintf("[%v] t:%v S:%v n:%v -> p:%v %v TS\n", len(runStats), event.time, srcPeer.node.index, node.index, peer.node.index, event.part))
										peer.setGiven(event.part)
										// numSendSuccess++
									} else {
										//logWrite(fmt.Sprintf("[%v] t:%v S:%v n:%v -> p:%v %v TF\n", len(runStats), event.time, srcPeer.node.index, node.index, peer.node.index, event.part))
										// numSendFailure++
									}
								}*/
							}
						}

					case EventDataResponse:
						event := _event.(EventDataResponse)
						peer := node.peers[event.src]

						// Adjust peer.wanted accordingly
						if event.rank <= maxAllowableRank {
							peer.setWants(event.part, true)
						} else {
							peer.setWants(event.part, false)
						}
					}

				}
			}

			if numEvents == 0 {
				numEventsZero++
			} else {
				numEventsZero = 0
			}
			// If network is full or numEventsZero > 3, quit.
			if countFull(nodes) == numNodes || numEventsZero > 3 {
				fmt.Printf("Done! took %v ms. Past: %v\n", timeMS, runStats)
				fillSum := 0.0
				for _, node := range nodes {
					fillSum += node.fill()
				}
				runStats = append(runStats, runStat{timeMS, fillSum / float64(numNodes), float64(numSendSuccess) / float64(numSendSuccess+numSendFailure), float64(numDups) / float64(numReceives)})
				for i := 0; i < 20; i++ {
					node := nodes[i]
					fmt.Printf("[%v] parts: %X (%f)\n", node.index, node.parts[:80], node.fill())
				}
				for i := 20; i < 2000; i += 200 {
					node := nodes[i]
					fmt.Printf("[%v] parts: %X (%f)\n", node.index, node.parts[:80], node.fill())
				}
				break
			} else {
				fmt.Printf("simulated %v ms. numEvents: %v Past: %v\n", timeMS, numEvents, runStats)
				for i := 0; i < 2; i++ {
					peer := nodes[0].peers[i]
					fmt.Printf("[0].[%v] wanted: %X\n", i, peer.wanted[:80])
					fmt.Printf("[0].[%v] given:  %X\n", i, peer.given[:80])
				}
				for i := 0; i < 5; i++ {
					node := nodes[i]
					fmt.Printf("[%v] parts: %X (%f)\n", node.index, node.parts[:80], node.fill())
				}
			}

			// Lets increment the timeMS now
			timeMS += latencyMS / 2

		} // end simulation
	} // forever loop
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
	if len(h.pq) == 0 {
		return nil
	}
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
