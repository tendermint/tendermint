package crawler

import (
	"fmt"
	"github.com/tendermint/tendermint/binary"
	rpctypes "github.com/tendermint/tendermint/rpc/core/types"
	rpcclient "github.com/tendermint/tendermint/rpc/core_client"
	"github.com/tendermint/tendermint/types"
	"time"

	"io/ioutil"
)

const (
	CheckQueueBufferSize  = 100
	NodeQueueBufferSize   = 100
	GetPeersTickerSeconds = 5
)

//---------------------------------------------------------------------------------------
// crawler.Node

// A node is a peer on the network
type Node struct {
	Host    string
	P2PPort uint16
	RPCPort uint16

	failed    int
	connected bool

	client *NodeClient

	LastSeen     time.Time
	ChainID      string
	BlockHeight  int
	BlockHistory map[int]time.Time // when we saw each block
	NetInfo      *rpctypes.ResponseNetInfo

	Validator bool

	// other peers we heard about this peer from
	heardFrom map[string]struct{}
}

func (n *Node) Address() string {
	return fmt.Sprintf("%s:%d", n.Host, n.RPCPort)
}

// Set the basic status and chain_id info for a node from RPC responses
func (n *Node) SetInfo(status *rpctypes.ResponseStatus, netinfo *rpctypes.ResponseNetInfo) {
	n.LastSeen = time.Now()
	n.ChainID = status.ChainID
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
		rpc: rpcclient.NewClient("http://"+addr, "JSONRPC"),
		ws:  rpcclient.NewWSClient("ws://" + addr + "/events"),
	}
}

// A simple wrapper for mediating access to the maps
type nodeInfo struct {
	host         string // the new nodes address
	port         uint16 // node's listening port
	from         string // the peer that told us about this node
	connected    bool   // move node from nodePool to nodes
	disconnected bool   // move node from nodes to nodePool
}

func (ni nodeInfo) unpack() (string, uint16, string, bool, bool) {
	return ni.host, ni.port, ni.from, ni.connected, ni.disconnected
}

// crawler.Node
//---------------------------------------------------------------------------------------
// crawler.Crawler

// A crawler has a local node, a set of potential nodes in the nodePool, and connected nodes.
// Maps are only accessed by one go-routine, mediated by the checkQueue
type Crawler struct {
	self   *Node
	client *NodeClient

	checkQueue chan nodeInfo
	nodePool   map[string]*Node
	nodes      map[string]*Node

	nodeQueue chan *Node
	quit      chan struct{}
}

// Create a new Crawler using the local RPC server at addr
func NewCrawler(host string, port uint16) *Crawler {
	return &Crawler{
		self:       &Node{Host: host, RPCPort: port, client: NewNodeClient(fmt.Sprintf("%s:%d", host, port))},
		checkQueue: make(chan nodeInfo, CheckQueueBufferSize),
		nodePool:   make(map[string]*Node),
		nodes:      make(map[string]*Node),
		nodeQueue:  make(chan *Node, NodeQueueBufferSize),
		quit:       make(chan struct{}),
	}
}

func (c *Crawler) Start() error {
	// connect to local node first, set info,
	// and fire peers onto the checkQueue
	if err := c.pollNode(c.self); err != nil {
		return err
	}

	// connect to weboscket, subscribe to local events
	// and run the read loop to listen for new blocks
	if r, err := c.self.client.ws.Dial(); err != nil {
		fmt.Println(r)
		b, _ := ioutil.ReadAll(r.Body)
		fmt.Println(string(b))
		return err
	}
	if err := c.self.client.ws.Subscribe(types.EventStringNewBlock()); err != nil {
		return err
	}
	go c.readLoop(c.self)

	// add ourselves to the nodes list
	c.nodes[c.self.Address()] = c.self

	// nodes we hear about get put on the checkQueue
	// by pollNode and are handled in the checkLoop.
	// if its a node we're not already connected to,
	// it gets put on the nodeQueue and
	// we attempt to connect in the connectLoop
	go c.checkLoop()
	go c.connectLoop()

	return nil
}

func (c *Crawler) Stop() {
	close(c.quit)
}

// listen for events from the node and ping it for peers on a ticker
func (c *Crawler) readLoop(node *Node) {
	wsChan := node.client.ws.Read()
	getPeersTicker := time.Tick(time.Second * GetPeersTickerSeconds)

	for {
		select {
		case wsMsg := <-wsChan:
			// update the node with his new info
			if err := c.consumeMessage(wsMsg, node); err != nil {
				// lost the node, put him back on the checkQueu
				c.checkNode(nodeInfo{
					host:         node.Host,
					port:         node.RPCPort,
					disconnected: true,
				})

			}
		case <-getPeersTicker:
			if err := c.pollNode(node); err != nil {
				// lost the node, put him back on the checkQueu
				c.checkNode(nodeInfo{
					host:         node.Host,
					port:         node.RPCPort,
					disconnected: true,
				})
			}
		case <-c.quit:
			return

		}
	}
}

func (c *Crawler) consumeMessage(wsMsg *rpcclient.WSMsg, node *Node) error {
	if wsMsg.Error != nil {
		return wsMsg.Error
	}
	// unmarshal block event
	var response struct {
		Event string
		Data  *types.Block
		Error string
	}
	var err error
	binary.ReadJSON(&response, wsMsg.Data, &err)
	if err != nil {
		return err
	}
	if response.Error != "" {
		return fmt.Errorf(response.Error)
	}
	block := response.Data

	node.LastSeen = time.Now()
	node.BlockHeight = block.Height
	node.BlockHistory[block.Height] = node.LastSeen

	return nil
}

// check nodes against the nodePool map one at a time
// acts as a mutex on nodePool and nodes
func (c *Crawler) checkLoop() {
	for {
		select {
		case ni := <-c.checkQueue:
			host, port, from, connected, disconnected := ni.unpack()
			addr := fmt.Sprintf("%s:%d", host, port)
			// check if we need to swap node between maps (eg. its connected or disconnected)
			// NOTE: once we hear about a node, we never forget ...
			if connected {
				n, _ := c.nodePool[addr]
				c.nodes[addr] = n
				delete(c.nodePool, addr)
				continue
			} else if disconnected {
				n, _ := c.nodes[addr]
				c.nodePool[addr] = n
				delete(c.nodes, addr)
				continue
			}

			// TODO: if address is badly formed
			// we should punish ni.from
			_ = from

			n, ok := c.nodePool[addr]
			// create the node if unknown
			if !ok {
				n = &Node{Host: host, RPCPort: port}
				c.nodePool[addr] = n
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

	if b, err := node.client.ws.Dial(); err != nil {
		fmt.Println("err on ws dial:", b, err)
		//  set failed, return
	}

	// remove from nodePool, add to nodes
	c.checkNode(nodeInfo{
		host:      node.Host,
		port:      node.RPCPort,
		connected: true,
	})

	if err := c.pollNode(node); err != nil {
		// TODO: we had a good ws con
		// but failed on rpc?!
		// try again or something ...
		// if we still fail, report and disconnect
	}

	fmt.Println("Successfully connected to node", node.Address())

	// blocks (until quit or err)
	c.readLoop(node)
}

func (c *Crawler) checkNode(ni nodeInfo) {
	c.checkQueue <- ni
}

func (c *Crawler) pollNode(node *Node) error {
	// get the status info
	status, err := node.client.rpc.Status()
	if err != nil {
		return err
	}

	// get peers and net info
	netinfo, err := node.client.rpc.NetInfo()
	if err != nil {
		return err
	}

	// set the info for the node
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
