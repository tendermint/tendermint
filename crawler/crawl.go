package crawler

import (
	"fmt"
	rpctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpcclient "github.com/tendermint/tendermint/rpc/core_client"
	types "github.com/tendermint/tendermint/types"
	"sync"
	"time"
)

const (
	CheckQueueBufferSize = 100
	NodeQueueBufferSize  = 100
)

//---------------------------------------------------------------------------------------
// crawler.Node

// A node is a peer on the network.
type Node struct {
	Host    string
	P2PPort uint16
	RPCPort uint16

	failed    int
	connected bool

	client *NodeClient

	LastSeen    time.Time
	GenesisHash []byte
	BlockHeight uint
	NetInfo     *rpctypes.ResponseNetInfo

	Validator bool

	// other peers we heard about this peer from
	heardFrom map[string]struct{}
}

func (n *Node) Address() string {
	return fmt.Sprintf("%s:%d", n.Host, n.RPCPort)
}

// Set the basic status and network info for a node from RPC responses
func (n *Node) SetInfo(status *rpctypes.ResponseStatus, netinfo *rpctypes.ResponseNetInfo) {
	n.LastSeen = time.Now()
	n.GenesisHash = status.GenesisHash
	n.BlockHeight = status.LatestBlockHeight
	n.NetInfo = netinfo
	// n.Validator
}

// A node client is used to talk to a node over rpc and websockets
type NodeClient struct {
	rpc rpcclient.Client
	ws  *rpcclient.WSClient
}

// Create a new client for the node at the given addr
func NewNodeClient(addr string) *NodeClient {
	return &NodeClient{
		rpc: rpcclient.NewClient(addr, "JSONRPC"),
		ws:  rpcclient.NewWSClient(addr),
	}
}

// A simple wrapper for mediating access to the maps
type nodeInfo struct {
	host    string // the new nodes address
	port    uint16
	from    string // the peer that told us about this node
	removed bool   // whether to remove from nodePool
}

func (ni nodeInfo) unpack() (string, uint16, string, bool) {
	return ni.host, ni.port, ni.from, ni.removed
}

// crawler.Node
//---------------------------------------------------------------------------------------
// crawler.Crawler

// A crawler has a local node, a set of potential nodes in the nodePool
// and connected nodes. Maps are only accessed by one go-routine, mediated through channels
type Crawler struct {
	self   *Node
	client *NodeClient

	checkQueue chan nodeInfo
	nodePool   map[string]*Node
	nodes      map[string]*Node

	nodeQueue chan *Node
	quit      chan struct{}

	// waits for checkQueue to empty
	// so we can re-poll all nodes
	wg sync.WaitGroup
}

// Create a new Crawler using the local RPC server at addr
func NewCrawler(host string, port uint16) *Crawler {
	return &Crawler{
		self:       &Node{Host: host, RPCPort: port},
		client:     NewNodeClient(fmt.Sprintf("%s:%d", host, port)),
		checkQueue: make(chan nodeInfo, CheckQueueBufferSize),
		nodePool:   make(map[string]*Node),
		nodes:      make(map[string]*Node),
		nodeQueue:  make(chan *Node, NodeQueueBufferSize),
		quit:       make(chan struct{}),
	}
}

func (c *Crawler) checkNode(ni nodeInfo) {
	c.wg.Add(1)
	c.checkQueue <- ni
}

func (c *Crawler) Start() error {
	// make sure we can connect
	// to our local node first
	status, err := c.client.rpc.Status()
	if err != nil {
		return err
	}

	// connect to weboscket and subscribe to local events
	if err = c.client.ws.Dial(); err != nil {
		return err
	}
	if err = c.client.ws.Subscribe(types.EventStringNewBlock()); err != nil {
		return err
	}
	go c.readLocalEvents()

	// add ourselves to the nodes list
	c.nodes[c.self.Address()] = c.self

	// get peers from local node
	netinfo, err := c.client.rpc.NetInfo()
	if err != nil {
		return err
	}

	// set the info for ourselves
	c.self.SetInfo(status, netinfo)

	// fire each peer on the checkQueue
	for _, p := range netinfo.Peers {
		c.checkNode(nodeInfo{
			host: p.Host,
			port: p.RPCPort,
		})
	}

	// nodes we hear about get put on the
	// checkQueue and are handled in the checkLoop
	// if its a node we're not already connected to,
	// it gets put it on the nodeQueue and
	// we attempt to connect in the connectLoop
	go c.checkLoop()
	go c.connectLoop()

	// finally, a routine with a ticker to poll nodes for peers
	go c.pollNodesRoutine()

	return nil
}

func (c *Crawler) Stop() {
	close(c.quit)
}

func (c *Crawler) readLocalEvents() {
	// read on our ws for NewBlocks
}

// check nodes against the nodePool map one at a time
// acts as a mutex on nodePool
func (c *Crawler) checkLoop() {
	c.wg.Add(1)
	for {
		// every time the loop restarts
		// it means we processed from the checkQueue
		// (except the first time, hence the extra wg.Add)
		c.wg.Done()
		select {
		case ni := <-c.checkQueue:
			host, port, from, removed := ni.unpack()
			addr := fmt.Sprintf("%s:%d", host, port)
			// check if the node should be removed
			// (eg. if its been connected to or abandoned)
			if removed {
				n, _ := c.nodePool[addr]
				if n.connected {
					c.nodes[addr] = n
				}
				delete(c.nodePool, addr)
				continue
			}

			// TODO: if address is badly formed
			// we should punish ni.from
			_ = from

			n, ok := c.nodePool[addr]
			// create the node if unknown
			if !ok {
				n = &Node{Host: host, RPCPort: port}
				c.nodes[addr] = n
			} else if n.connected {
				// should be removed soon
				continue
			}

			// queue it for connecting to
			c.nodeQueue <- n

		case <-c.quit:
			return
		}
	}
}

// read off the nodeQueue and attempt to connect to nodes
func (c *Crawler) connectLoop() {
	for {
		select {
		case node := <-c.nodeQueue:
			go c.connectToNode(node)
		case <-c.quit:
			// close all connections
			for addr, node := range c.nodes {
				_, _ = addr, node
				// TODO: close conn
			}
			return
		}
	}
}

func (c *Crawler) connectToNode(node *Node) {

	addr := node.Address()
	node.client = NewNodeClient(addr)

	if err := node.client.ws.Dial(); err != nil {
		//  set failed, return
	}

	// remove from nodePool, add to nodes
	c.checkNode(nodeInfo{
		host:    node.Host,
		port:    node.RPCPort,
		removed: true,
	})

	c.pollNode(node)

	// TODO: read loop
}

func (c *Crawler) pollNode(node *Node) error {
	status, err := node.client.rpc.Status()
	if err != nil {
		return err
	}

	// get peers
	netinfo, err := node.client.rpc.NetInfo()
	if err != nil {
		return err
	}

	node.SetInfo(status, netinfo)

	// fire each peer on the checkQueue
	for _, p := range netinfo.Peers {
		c.checkNode(nodeInfo{
			host: p.Host,
			port: p.RPCPort,
			from: node.Address(),
		})
	}
	return nil
}

// wait for the checkQueue to empty and poll all the nodes
func (c *Crawler) pollNodesRoutine() {
	for {
		c.wg.Wait()

		ticker := time.Tick(time.Second)
		// wait a few seconds to make sure we really have nothing
		time.Sleep(time.Second * 5)
		ch := make(chan struct{})
		go func() {
			c.wg.Wait()
			ch <- struct{}{}
		}()

		//
		select {
		case <-ticker:
			// the checkQueue has filled up again, move on
			continue
		case <-ch:
			// the checkQueue is legit empty,
			// TODO: poll the nodes!

		case <-c.quit:
			return
		}
	}

}
