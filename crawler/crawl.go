package crawler

import (
	"fmt"
	"time"

	. "github.com/eris-ltd/tendermint/common"
	ctypes "github.com/eris-ltd/tendermint/rpc/core/types"
	cclient "github.com/eris-ltd/tendermint/rpc/core_client"
	"github.com/eris-ltd/tendermint/types"
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
	NetInfo      *ctypes.ResultNetInfo

	Validator bool

	// other peers we heard about this peer from
	heardFrom map[string]struct{}
}

func (n *Node) Address() string {
	return fmt.Sprintf("%s:%d", n.Host, n.RPCPort)
}

// Set the basic status and chain_id info for a node from RPC responses
func (n *Node) SetInfo(status *ctypes.ResultStatus, netinfo *ctypes.ResultNetInfo) {
	n.LastSeen = time.Now()
	n.ChainID = status.NodeInfo.ChainID
	n.BlockHeight = status.LatestBlockHeight
	n.NetInfo = netinfo
	// n.Validator
}

// A node client is used to talk to a node over rpc and websockets
type NodeClient struct {
	rpc cclient.Client
	ws  *cclient.WSClient
}

// Create a new client for the node at the given addr
func NewNodeClient(addr string) *NodeClient {
	return &NodeClient{
		rpc: cclient.NewClient("http://"+addr, "JSONRPC"),
		ws:  cclient.NewWSClient("ws://" + addr + "/events"),
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
	QuitService

	self   *Node
	client *NodeClient

	checkQueue chan nodeInfo
	nodePool   map[string]*Node
	nodes      map[string]*Node

	nodeQueue chan *Node
}

// Create a new Crawler using the local RPC server at addr
func NewCrawler(host string, port uint16) *Crawler {
	crawler := &Crawler{
		self:       &Node{Host: host, RPCPort: port, client: NewNodeClient(fmt.Sprintf("%s:%d", host, port))},
		checkQueue: make(chan nodeInfo, CheckQueueBufferSize),
		nodePool:   make(map[string]*Node),
		nodes:      make(map[string]*Node),
		nodeQueue:  make(chan *Node, NodeQueueBufferSize),
	}
	crawler.QuitService = *NewQuitService(log, "Crawler", crawler)
	return crawler
}

func (c *Crawler) OnStart() error {
	// connect to local node first, set info,
	// and fire peers onto the checkQueue
	if err := c.pollNode(c.self); err != nil {
		return err
	}

	// connect to weboscket, subscribe to local events
	// and run the read loop to listen for new blocks
	_, err := c.self.client.ws.Start()
	if err != nil {
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

// listen for events from the node and ping it for peers on a ticker
func (c *Crawler) readLoop(node *Node) {
	eventsCh := node.client.ws.EventsCh
	getPeersTicker := time.Tick(time.Second * GetPeersTickerSeconds)

	for {
		select {
		case eventMsg := <-eventsCh:
			// update the node with his new info
			if err := c.consumeMessage(eventMsg, node); err != nil {
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
		case <-c.Quit:
			return

		}
	}
}

func (c *Crawler) consumeMessage(eventMsg ctypes.ResultEvent, node *Node) error {
	block := eventMsg.Data.(*types.EventDataNewBlock).Block
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

		case <-c.Quit:
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
		case <-c.Quit:
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
	_, err := node.client.ws.Start()
	if err != nil {
		fmt.Println("err on ws start:", err)
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
