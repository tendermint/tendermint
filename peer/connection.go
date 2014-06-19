package peer

import (
    . "github.com/tendermint/tendermint/common"
    . "github.com/tendermint/tendermint/binary"
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

    mtx             sync.Mutex
    outQueue        chan ByteSlice
    conn            net.Conn
    quit            chan struct{}
    disconnected    bool

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

func (c *Connection) QueueMessage(msg ByteSlice) bool {
    c.mtx.Lock(); defer c.mtx.Unlock()
    if c.disconnected { return false }
    select {
    case c.outQueue <- msg:
        return true
    default: // buffer full
        return false
    }
}

func (c *Connection) Start() {
    go c.outHandler()
    go c.inHandler()
}

func (c *Connection) Disconnect() {
    c.mtx.Lock(); defer c.mtx.Unlock()
    close(c.quit)
    c.conn.Close()
    c.pingDebouncer.Stop()
    // do not close c.pong
    c.disconnected = true
}

func (c *Connection) flush() {
    // TODO flush? (turn off nagel, turn back on, etc)
}

func (c *Connection) outHandler() {

    FOR_LOOP:
    for {
        select {
        case <-c.pingDebouncer.Ch:
            PACKET_TYPE_PING.WriteTo(c.conn)
        case outMsg := <-c.outQueue:
            _, err := outMsg.WriteTo(c.conn)
            if err != nil { Panicf("TODO: handle error %v", err) }
        case <-c.pong:
            PACKET_TYPE_PONG.WriteTo(c.conn)
        case <-c.quit:
            break FOR_LOOP
        }
        c.flush()
    }

    // cleanup
    for _ = range c.outQueue {
        // do nothing but drain.
    }
}

func (c *Connection) inHandler() {
    defer func() {
        if e := recover(); e != nil {
            // Get stack trace
            buf := make([]byte, 1<<16)
            runtime.Stack(buf, false)
            // TODO do proper logging
            fmt.Printf("Disconnecting due to error:\n\n%v\n", string(buf))
            c.Disconnect()
        }
    }()

    //FOR_LOOP:
    for {
        msgType := ReadUInt8(c.conn)

        switch msgType {
        case PACKET_TYPE_PING:
            c.pong <- struct{}{}
        case PACKET_TYPE_PONG:
            // do nothing
        case PACKET_TYPE_MSG:
            ReadByteSlice(c.conn)
        default:
            Panicf("Unknown message type %v", msgType)
        }
        c.pingDebouncer.Reset()
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
