package peer

import (
    . "github.com/tendermint/tendermint/common"
    . "github.com/tendermint/tendermint/binary"
    "atomic"
    "sync"
    "net"
    "runtime"
    "fmt"
    "time"
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
    stopped         int32
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
    go c.outHandler()
    go c.inHandler()
}

func (c *Connection) Stop() {
    if atomic.SwapAndCompare(&c.stopped, 0, 1) {
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

func (c *Connection) flush() {
    // TODO flush? (turn off nagel, turn back on, etc)
}

func (c *Connection) outHandler() {

    FOR_LOOP:
    for {
        var err error
        select {
        case <-c.pingDebouncer.Ch:
            _, err = PACKET_TYPE_PING.WriteTo(c.conn)
        case outMsg := <-c.outQueue:
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

}

func (c *Connection) inHandler() {

    FOR_LOOP:
    for {
        msgType, err := ReadUInt8Safe(c.conn)

        if err != nil {
            if atomic.LoadUint32(&c.stopped) != 1 {
                log.Infof("Connection %v failed @ inHandler", c)
                c.Stop()
            }
            break FOR_LOOP
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
            // TODO
            
        default:
            Panicf("Unknown message type %v", msgType)
        }

        c.pingDebouncer.Reset()
    }

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
