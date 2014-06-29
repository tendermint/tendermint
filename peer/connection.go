package peer

import (
    . "github.com/tendermint/tendermint/common"
    . "github.com/tendermint/tendermint/binary"
    "sync/atomic"
    "net"
    "time"
    "fmt"
)

const (
    OUT_QUEUE_SIZE = 50
    IDLE_TIMEOUT_MINUTES = 5
    PING_TIMEOUT_MINUTES = 2
)

/* Connnection */
type Connection struct {
    ioStats         IOStats

    outQueue        chan ByteSlice // never closes.
    conn            net.Conn
    quit            chan struct{}
    stopped         uint32
    pingDebouncer   *Debouncer
    pong            chan struct{}
}

var (
    PACKET_TYPE_PING = UInt8(0x00)
    PACKET_TYPE_PONG = UInt8(0x01)
    PACKET_TYPE_MSG =  UInt8(0x10)
)

func NewConnection(conn net.Conn) *Connection {
    return &Connection{
        outQueue:       make(chan ByteSlice, OUT_QUEUE_SIZE),
        conn:           conn,
        quit:           make(chan struct{}),
        pingDebouncer:  NewDebouncer(PING_TIMEOUT_MINUTES * time.Minute),
        pong:           make(chan struct{}),
    }
}

// returns true if successfully queued,
// returns false if connection was closed.
// blocks.
func (c *Connection) QueueOut(msg ByteSlice) bool {
    select {
    case c.outQueue <- msg:
        return true
    case <-c.quit:
        return false
    }
}

func (c *Connection) Start() {
    log.Debugf("Starting %v", c)
    go c.outHandler()
    go c.inHandler()
}

func (c *Connection) Stop() {
    if atomic.CompareAndSwapUint32(&c.stopped, 0, 1) {
        log.Debugf("Stopping %v", c)
        close(c.quit)
        c.conn.Close()
        c.pingDebouncer.Stop()
        // We can't close pong safely here because
        // inHandler may write to it after we've stopped.
        // Though it doesn't need to get closed at all,
        // we close it @ inHandler.
        // close(c.pong)
    }
}

func (c *Connection) LocalAddress() *NetAddress {
    return NewNetAddress(c.conn.LocalAddr())
}

func (c *Connection) RemoteAddress() *NetAddress {
    return NewNetAddress(c.conn.RemoteAddr())
}

func (c *Connection) String() string {
    return fmt.Sprintf("Connection{%v}", c.conn.RemoteAddr())
}

func (c *Connection) flush() {
    // TODO flush? (turn off nagel, turn back on, etc)
}

func (c *Connection) outHandler() {
    log.Tracef("Connection %v outHandler", c)

    FOR_LOOP:
    for {
        var err error
        select {
        case <-c.pingDebouncer.Ch:
            _, err = PACKET_TYPE_PING.WriteTo(c.conn)
        case outMsg := <-c.outQueue:
            log.Tracef("Found msg from outQueue. Writing msg to underlying connection")
            _, err = PACKET_TYPE_MSG.WriteTo(c.conn)
            if err != nil { break }
            _, err = outMsg.WriteTo(c.conn)
        case <-c.pong:
            _, err = PACKET_TYPE_PONG.WriteTo(c.conn)
        case <-c.quit:
            break FOR_LOOP
        }

        if err != nil {
            log.Infof("Connection %v failed @ outHandler:\n%v", c, err)
            c.Stop()
            break FOR_LOOP
        }

        c.flush()
    }

    log.Tracef("Connection %v outHandler done", c)
    // cleanup
}

func (c *Connection) inHandler() {
    log.Tracef("Connection %v inHandler", c)

    FOR_LOOP:
    for {
        msgType, err := ReadUInt8Safe(c.conn)
        if err != nil {
            if atomic.LoadUint32(&c.stopped) != 1 {
                log.Infof("Connection %v failed @ inHandler", c)
                c.Stop()
            }
            break FOR_LOOP
        } else {
            log.Tracef("Found msgType %v", msgType)
        }

        switch msgType {
        case PACKET_TYPE_PING:
            c.pong <- struct{}{}
        case PACKET_TYPE_PONG:
            // do nothing
        case PACKET_TYPE_MSG:
            msg, err := ReadByteSliceSafe(c.conn)
            if err != nil {
                if atomic.LoadUint32(&c.stopped) != 1 {
                    log.Infof("Connection %v failed @ inHandler", c)
                    c.Stop()
                }
                break FOR_LOOP
            }
            // What to do?
            // XXX
            XXX well, we need to push it into the channel or something.
            or at least provide an inQueue.
            log.Tracef("%v", msg)
        default:
            Panicf("Unknown message type %v", msgType)
        }

        c.pingDebouncer.Reset()
    }

    log.Tracef("Connection %v inHandler done", c)
    // cleanup
    close(c.pong)
    for _ = range c.pong {
        // drain
    }
}


/* IOStats */
type IOStats struct {
    TimeConnected   Time
    LastSent        Time
    LastRecv        Time
    BytesRecv       UInt64
    BytesSent       UInt64
    MsgsRecv        UInt64
    MsgsSent        UInt64
}
