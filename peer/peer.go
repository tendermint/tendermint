package peer

import (
    "bytes"
    "container/list"
    "fmt"
    "github.com/davecgh/go-spew/spew"
    "github.com/tendermint/btcwire"
    "net"
    "strconv"
    "sync"
    "sync/atomic"
    "time"
)

const (
    // max protocol version the peer supports.
    maxProtocolVersion = 70001

    // number of elements the output channels use.
    outputBufferSize = 50

    // number of seconds of inactivity before we timeout a peer
    // that hasn't completed the initial version negotiation.
    negotiateTimeoutSeconds = 30

    // number of minutes of inactivity before we time out a peer.
    idleTimeoutMinutes = 5

    // number of minutes since we last sent a message
    // requiring a reply before we will ping a host.
    pingTimeoutMinutes = 2
)

var (
    userAgentName = "tendermintd"
    userAgentVersion = fmt.Sprintf("%d.%d.%d", appMajor, appMinor, appPatch)
)

// zeroHash is the zero value hash (all zeros).  It is defined as a convenience.
var zeroHash btcwire.ShaHash

// minUint32 is a helper function to return the minimum of two uint32s.
// This avoids a math import and the need to cast to floats.
func minUint32(a, b uint32) uint32 {
    if a < b {
        return a
    }
    return b
}

// TODO(davec): Rename and comment this
type outMsg struct {
    msg      btcwire.Message
    doneChan chan bool
}

/*
The overall data flow is split into 2 goroutines.

Inbound messages are read via the inHandler goroutine and generally
dispatched to their own handler.

Outbound messages are queued via QueueMessage.
*/
type peer struct {
    server              *server
    addr                *NetAddress
    inbound             bool
    persistent          bool

    started             bool        // atomic
    quit                chan bool

    conn                net.Conn
    connMtx             sync.Mutex
    disconnected        bool        // atomic && protected by connMtx
    knownAddresses      map[string]bool
    outputQueue         chan outMsg

    statMtx             sync.Mutex  // protects all below here.
    protocolVersion     uint32
    timeConnected       time.Time
    lastSend            time.Time
    lastRecv            time.Time
    bytesReceived       uint64
    bytesSent           uint64
    userAgent           string
    lastPingNonce       uint64      // Set to nonce if we have a pending ping.
    lastPingTime        time.Time   // Time we sent last ping.
    lastPingMicros      int64       // Time for last ping to return.
}

// String returns the peer's address and directionality as a human-readable
// string.
func (p *peer) String() string {
    return fmt.Sprintf("%s (%s)", p.addr.String(), directionString(p.inbound))
}

// VersionKnown returns the whether or not the version of a peer is known locally.
// It is safe for concurrent access.
func (p *peer) VersionKnown() bool {
    p.statMtx.Lock(); defer p.statMtx.Unlock()

    return p.protocolVersion != 0
}

// ProtocolVersion returns the peer protocol version in a manner that is safe
// for concurrent access.
func (p *peer) ProtocolVersion() uint32 {
    p.statMtx.Lock(); defer p.statMtx.Unlock()

    return p.protocolVersion
}

// pushVersionMsg sends a version message to the connected peer using the
// current state.
func (p *peer) pushVersionMsg() {
    _, blockNum, err := p.server.db.NewestSha()
    if err != nil { panic(err) }

    // Version message.
    // TODO: DisableListen -> send zero address
    msg := btcwire.NewMsgVersion(
        p.server.addrManager.getBestLocalAddress(p.addr), p.addr,
        p.server.nonce, int32(blockNum))
    msg.AddUserAgent(userAgentName, userAgentVersion)

    // Advertise our max supported protocol version.
    msg.ProtocolVersion = maxProtocolVersion

    p.QueueMessage(msg, nil)
}

// handleVersionMsg is invoked when a peer receives a version bitcoin message
// and is used to negotiate the protocol version details as well as kick start
// the communications.
func (p *peer) handleVersionMsg(msg *btcwire.MsgVersion) {
    // Detect self connections.
    if msg.Nonce == p.server.nonce {
        peerLog.Debugf("Disconnecting peer connected to self %s", p)
        p.Disconnect()
        return
    }

    p.statMtx.Lock() // Updating a bunch of stats.
    // Limit to one version message per peer.
    if p.protocolVersion != 0 {
        p.logError("Only one version message per peer is allowed %s.", p)
        p.statMtx.Unlock()
        p.Disconnect()
        return
    }

    // Negotiate the protocol version.
    p.protocolVersion = minUint32(p.protocolVersion, uint32(msg.ProtocolVersion))
    peerLog.Debugf("Negotiated protocol version %d for peer %s", p.protocolVersion, p)

    // Set the remote peer's user agent.
    p.userAgent = msg.UserAgent

    p.statMtx.Unlock()

    // Inbound connections.
    if p.inbound {
        // Send version.
        p.pushVersionMsg()
    }

    // Send verack.
    p.QueueMessage(btcwire.NewMsgVerAck(), nil)

    if p.inbound {
        // A peer might not be advertising the same address that it
        // actually connected from.  One example of why this can happen
        // is with NAT.  Only add the address to the address manager if
        // the addresses agree.
        if msg.AddrMe.String() == p.addr.String() {
            p.server.addrManager.AddAddress(p.addr, p.addr)
        }
    } else {
        // Request known addresses from the remote peer.
        if !cfg.SimNet && p.server.addrManager.NeedMoreAddresses() {
            p.QueueMessage(btcwire.NewMsgGetAddr(), nil)
        }
    }

    // Mark the address as a known good address.
    p.server.addrManager.MarkGood(p.addr)

    // Signal the block manager this peer is a new sync candidate.
    p.server.blockManager.NewPeer(p)

    // TODO: Relay alerts.
}


// handleGetAddrMsg is invoked when a peer receives a getaddr bitcoin message
// and is used to provide the peer with known addresses from the address
// manager.
func (p *peer) handleGetAddrMsg(msg *btcwire.MsgGetAddr) {
    // Don't return any addresses when running on the simulation test
    // network.  This helps prevent the network from becoming another
    // public test network since it will not be able to learn about other
    // peers that have not specifically been provided.
    if cfg.SimNet {
        return
    }

    // Get the current known addresses from the address manager.
    addrCache := p.server.addrManager.AddressCache()

    // Push the addresses.
    p.pushAddrMsg(addrCache)
}

// pushAddrMsg sends one, or more, addr message(s) to the connected peer using
// the provided addresses.
func (p *peer) pushAddrMsg(addresses []*NetAddress) {
    // Nothing to send.
    if len(addresses) == 0 { return }

    numAdded := 0
    msg := btcwire.NewMsgAddr()
    for _, addr := range addresses {
        // Filter addresses the peer already knows about.
        if p.knownAddresses[addr.String()] {
            continue
        }

        // Add the address to the message.
        err := msg.AddAddress(addr)
        if err != nil { panic(err) } // XXX remove error condition
        numAdded++

        // Split into multiple messages as needed.
        if numAdded > 0 && numAdded%btcwire.MaxAddrPerMsg == 0 {
            p.QueueMessage(msg, nil)

            // NOTE: This needs to be a new address message and not
            // simply call ClearAddresses since the message is a
            // pointer and queueing it does not make a copy.
            msg = btcwire.NewMsgAddr()
        }
    }

    // Send message with remaining addresses if needed.
    if numAdded%btcwire.MaxAddrPerMsg != 0 {
        p.QueueMessage(msg, nil)
    }
}

// handleAddrMsg is invoked when a peer receives an addr bitcoin message and
// is used to notify the server about advertised addresses.
func (p *peer) handleAddrMsg(msg *btcwire.MsgAddr) {
    // Ignore addresses when running on the simulation test network.  This
    // helps prevent the network from becoming another public test network
    // since it will not be able to learn about other peers that have not
    // specifically been provided.
    if cfg.SimNet {
        return
    }

    // A message that has no addresses is invalid.
    if len(msg.AddrList) == 0 {
        p.logError("Command [%s] from %s does not contain any addresses", msg.Command(), p)
        p.Disconnect()
        return
    }

    for _, addr := range msg.AddrList {
        // Set the timestamp to 5 days ago if it's more than 24 hours
        // in the future so this address is one of the first to be
        // removed when space is needed.
        now := time.Now()
        if addr.Timestamp.After(now.Add(time.Minute * 10)) {
            addr.Timestamp = now.Add(-1 * time.Hour * 24 * 5)
        }

        // Add address to known addresses for this peer.
        p.knownAddresses[addr.String()] = true
    }

    // Add addresses to server address manager.  The address manager handles
    // the details of things such as preventing duplicate addresses, max
    // addresses, and last seen updates.
    // XXX bitcoind gives a 2 hour time penalty here, do we want to do the
    // same?
    p.server.addrManager.AddAddresses(msg.AddrList, p.addr)
}

func (p *peer) handlePingMsg(msg *btcwire.MsgPing) {
    // Include nonce from ping so pong can be identified.
    p.QueueMessage(btcwire.NewMsgPong(msg.Nonce), nil)
}

func (p *peer) handlePongMsg(msg *btcwire.MsgPong) {
    p.statMtx.Lock(); defer p.statMtx.Unlock()

    // Arguably we could use a buffered channel here sending data
    // in a fifo manner whenever we send a ping, or a list keeping track of
    // the times of each ping. For now we just make a best effort and
    // only record stats if it was for the last ping sent. Any preceding
    // and overlapping pings will be ignored. It is unlikely to occur
    // without large usage of the ping rpc call since we ping
    // infrequently enough that if they overlap we would have timed out
    // the peer.
    if p.lastPingNonce != 0 && msg.Nonce == p.lastPingNonce {
        p.lastPingMicros = time.Now().Sub(p.lastPingTime).Nanoseconds()
        p.lastPingMicros /= 1000 // convert to usec.
        p.lastPingNonce = 0
    }
}

// readMessage reads the next bitcoin message from the peer with logging.
func (p *peer) readMessage() (btcwire.Message, []byte, error) {
    n, msg, buf, err := btcwire.ReadMessageN(p.conn, p.ProtocolVersion())
    p.statMtx.Lock()
    p.bytesReceived += uint64(n)
    p.statMtx.Unlock()
    p.server.AddBytesReceived(uint64(n))
    if err != nil {
        return nil, nil, err
    }

    // Use closures to log expensive operations so they are only run when
    // the logging level requires it.
    peerLog.Debugf("%v", newLogClosure(func() string {
        // Debug summary of message.
        summary := messageSummary(msg)
        if len(summary) > 0 {
            summary = " (" + summary + ")"
        }
        return fmt.Sprintf("Received %v%s from %s", msg.Command(), summary, p)
    }))
    peerLog.Tracef("%v", newLogClosure(func() string {
        return spew.Sdump(msg)
    }))
    peerLog.Tracef("%v", newLogClosure(func() string {
        return spew.Sdump(buf)
    }))

    return msg, buf, nil
}

// writeMessage sends a bitcoin Message to the peer with logging.
func (p *peer) writeMessage(msg btcwire.Message) {
    if p.Disconnected() { return }

    if !p.VersionKnown() {
        switch msg.(type) {
        case *btcwire.MsgVersion:
            // This is OK.
        default:
            // We drop all messages other than version if we
            // haven't done the handshake already.
            return
        }
    }

    // Use closures to log expensive operations so they are only run when
    // the logging level requires it.
    peerLog.Debugf("%v", newLogClosure(func() string {
        // Debug summary of message.
        summary := messageSummary(msg)
        if len(summary) > 0 {
            summary = " (" + summary + ")"
        }
        return fmt.Sprintf("Sending %v%s to %s", msg.Command(), summary, p)
    }))
    peerLog.Tracef("%v", newLogClosure(func() string {
        return spew.Sdump(msg)
    }))
    peerLog.Tracef("%v", newLogClosure(func() string {
        var buf bytes.Buffer
        err := btcwire.WriteMessage(&buf, msg, p.ProtocolVersion())
        if err != nil {
            return err.Error()
        }
        return spew.Sdump(buf.Bytes())
    }))

    // Write the message to the peer.
    n, err := btcwire.WriteMessageN(p.conn, msg, p.ProtocolVersion())
    p.statMtx.Lock()
    p.bytesSent += uint64(n)
    p.statMtx.Unlock()
    p.server.AddBytesSent(uint64(n))
    if err != nil {
        p.Disconnect()
        p.logError("Can't send message to %s: %v", p, err)
        return
    }
}


// inHandler handles all incoming messages for the peer.  It must be run as a
// goroutine.
func (p *peer) inHandler() {
    // Peers must complete the initial version negotiation within a shorter
    // timeframe than a general idle timeout.  The timer is then reset below
    // to idleTimeoutMinutes for all future messages.
    idleTimer := time.AfterFunc(negotiateTimeoutSeconds*time.Second, func() {
        if p.VersionKnown() {
            peerLog.Warnf("Peer %s no answer for %d minutes, disconnecting", p, idleTimeoutMinutes)
        }
        p.Disconnect()
    })
out:
    for !p.Disconnected() {
        rmsg, buf, err := p.readMessage()
        // Stop the timer now, if we go around again we will reset it.
        idleTimer.Stop()
        if err != nil {
            if !p.Disconnected() {
                p.logError("Can't read message from %s: %v", p, err)
            }
            break out
        }
        p.statMtx.Lock()
        p.lastRecv = time.Now()
        p.statMtx.Unlock()

        // Ensure version message comes first.
        if _, ok := rmsg.(*btcwire.MsgVersion); !ok && !p.VersionKnown() {
            p.logError("A version message must precede all others")
            break out
        }

        // Handle each supported message type.
        markGood := false
        switch msg := rmsg.(type) {
        case *btcwire.MsgVersion:
            p.handleVersionMsg(msg)

        case *btcwire.MsgVerAck:
            // Do nothing.

        case *btcwire.MsgGetAddr:
            p.handleGetAddrMsg(msg)

        case *btcwire.MsgAddr:
            p.handleAddrMsg(msg)
            markGood = true

        case *btcwire.MsgPing:
            p.handlePingMsg(msg)
            markGood = true

        case *btcwire.MsgPong:
            p.handlePongMsg(msg)

        case *btcwire.MsgAlert:
            p.server.BroadcastMessage(msg, p)

        case *btcwire.MsgNotFound:
            // TODO(davec): Ignore this for now, but ultimately
            // it should probably be used to detect when something
            // we requested needs to be re-requested from another
            // peer.

        default:
            peerLog.Debugf("Received unhandled message of type %v: Fix Me", rmsg.Command())
        }

        // Mark the address as currently connected and working as of
        // now if one of the messages that trigger it was processed.
        if markGood && !p.Disconnected() {
            if p.addr == nil {
                peerLog.Warnf("we're getting stuff before we got a version message. that's bad")
                continue
            }
            p.server.addrManager.MarkGood(p.addr)
        }
        // ok we got a message, reset the timer.
        // timer just calls p.Disconnect() after logging.
        idleTimer.Reset(idleTimeoutMinutes * time.Minute)
    }

    idleTimer.Stop()

    // Ensure connection is closed and notify the server that the peer is done.
    p.Disconnect()
    p.server.donePeers <- p

    // Only tell block manager we are gone if we ever told it we existed.
    if p.VersionKnown() {
        p.server.blockManager.DonePeer(p)
    }

    peerLog.Tracef("Peer input handler done for %s", p)
}

// outHandler handles all outgoing messages for the peer.  It must be run as a
// goroutine.  It uses a buffered channel to serialize output messages while
// allowing the sender to continue running asynchronously.
func (p *peer) outHandler() {
    pingTimer := time.AfterFunc(pingTimeoutMinutes*time.Minute, func() {
        nonce, err := btcwire.RandomUint64()
        if err != nil {
            peerLog.Errorf("Not sending ping on timeout to %s: %v",
                p, err)
            return
        }
        p.QueueMessage(btcwire.NewMsgPing(nonce), nil)
    })
out:
    for {
        select {
        case msg := <-p.outputQueue:
            // If the message is one we should get a reply for
            // then reset the timer, we only want to send pings
            // when otherwise we would not receive a reply from
            // the peer.
            peerLog.Tracef("%s: received from outputQueue", p)
            reset := true
            switch m := msg.msg.(type) {
            case *btcwire.MsgVersion:
                // should get an ack
            case *btcwire.MsgGetAddr:
                // should get addresses
            case *btcwire.MsgPing:
                // expects pong
                // Also set up statistics.
                p.statMtx.Lock()
                p.lastPingNonce = m.Nonce
                p.lastPingTime = time.Now()
                p.statMtx.Unlock()
            default:
                // Not one of the above, no sure reply.
                // We want to ping if nothing else
                // interesting happens.
                reset = false
            }
            if reset {
                pingTimer.Reset(pingTimeoutMinutes * time.Minute)
            }
            p.writeMessage(msg.msg)
            p.statMtx.Lock()
            p.lastSend = time.Now()
            p.statMtx.Unlock()
            if msg.doneChan != nil {
                msg.doneChan <- true
            }

        case <-p.quit:
            break out
        }
    }

    pingTimer.Stop()

    // Drain outputQueue
    for msg := range p.outputQueue {
        if msg.doneChan != nil {
            msg.doneChan <- false
        }
    }
    peerLog.Tracef("Peer output handler done for %s", p)
}

// QueueMessage adds the passed bitcoin message to the peer outputQueue.  It
// uses a buffered channel to communicate with the output handler goroutine so
// it is automatically rate limited and safe for concurrent access.
func (p *peer) QueueMessage(msg btcwire.Message, doneChan chan bool) {
    // Avoid risk of deadlock if goroutine already exited. The goroutine
    // we will be sending to hangs around until it knows for a fact that
    // it is marked as disconnected. *then* it drains the channels.
    if p.Disconnected() {
        // avoid deadlock...
        if doneChan != nil {
            go func() {
                doneChan <- false
            }()
        }
        return
    }
    p.outputQueue <- outMsg{msg: msg, doneChan: doneChan}
}

// True if is (or will become) disconnected.
func (p *peer) Disconnected() bool {
    return atomic.LoadInt32(&p.disconnected) == 1
}

// Disconnects the peer by closing the connection.  It also sets
// a flag so the impending shutdown can be detected.
func (p *peer) Disconnect() {
    p.connMtx.Lock(); defer p.connMtx.Unlock()
    // did we win the race?
    if atomic.AddInt32(&p.disconnected, 1) != 1 {
        return
    }
    peerLog.Tracef("disconnecting %s", p)
    close(p.quit)
    if p.conn != nil {
        p.conn.Close()
    }
}

// Sets the connection & starts
func (p *peer) StartWithConnection(conn *net.Conn) {
    p.connMtx.Lock(); defer p.connMtx.Unlock()
    if p.conn != nil { panic("Conn already set") }
    if atomic.LoadInt32(&p.disconnected) == 1 { return }
    peerLog.Debugf("Connected to %s", conn.RemoteAddr())
    p.timeConnected = time.Now()
    p.conn = conn
    p.Start()
}

// Start begins processing input and output messages.  It also sends the initial
// version message for outbound connections to start the negotiation process.
func (p *peer) Start() error {
    // Already started?
    if atomic.AddInt32(&p.started, 1) != 1 {
        return nil
    }

    peerLog.Tracef("Starting peer %s", p)

    // Send an initial version message if this is an outbound connection.
    if !p.inbound {
        p.pushVersionMsg()
    }

    // Start processing input and output.
    go p.inHandler()
    go p.outHandler()

    return nil
}

// Shutdown gracefully shuts down the peer by disconnecting it.
func (p *peer) Shutdown() {
    peerLog.Tracef("Shutdown peer %s", p)
    p.Disconnect()
}

// newPeerBase returns a new base peer for the provided server and inbound flag.
// This is used by the newInboundPeer and newOutboundPeer functions to perform
// base setup needed by both types of peers.
func newPeerBase(s *server, inbound bool) *peer {
    p := peer{
        server:          s,
        protocolVersion: maxProtocolVersion,
        inbound:         inbound,
        knownAddresses:  make(map[string]bool),
        outputQueue:     make(chan outMsg, outputBufferSize),
        quit:            make(chan bool),
    }
    return &p
}

// newPeer returns a new inbound bitcoin peer for the provided server and
// connection.  Use Start to begin processing incoming and outgoing messages.
func newInboundPeer(s *server, conn net.Conn) *peer {
    addr := NewNetAddress(conn.RemoteAddr())
    // XXX What if p.addr doesn't match (to be) reported addr due to NAT?
    s.addrManager.MarkAttempt(addr)

    p := newPeerBase(s, true)
    p.conn = conn
    p.addr = addr
    p.timeConnected = time.Now()
    return p
}

// newOutbountPeer returns a new outbound bitcoin peer for the provided server and
// address and connects to it asynchronously. If the connection is successful
// then the peer will also be started.
func newOutboundPeer(s *server, addr *NetAddress, persistent bool) *peer {
    p := newPeerBase(s, false)
    p.addr = addr
    p.persistent = persistent

    go func() {
        // Mark this as one attempt, regardless of # of reconnects.
        s.addrManager.MarkAttempt(p.addr)
        retryCount := 0
        // Attempt to connect to the peer.  If the connection fails and
        // this is a persistent connection, retry after the retry
        // interval.
        for {
            peerLog.Debugf("Attempting to connect to %s", addr)
            conn, err := addr.Dial()
            if err == nil {
                p.StartWithConnection(conn)
                return
            } else {
                retryCount++
                peerLog.Debugf("Failed to connect to %s: %v", addr, err)
                if !persistent {
                    p.server.donePeers <- p
                    return
                }
                scaledInterval := connectionRetryInterval.Nanoseconds() * retryCount / 2
                scaledDuration := time.Duration(scaledInterval)
                peerLog.Debugf("Retrying connection to %s in %s", addr, scaledDuration)
                time.Sleep(scaledDuration)
                continue
            }
        }
    }()
    return p
}

// logError makes sure that we only log errors loudly on user peers.
func (p *peer) logError(fmt string, args ...interface{}) {
    if p.persistent {
        peerLog.Errorf(fmt, args...)
    } else {
        peerLog.Debugf(fmt, args...)
    }
}
