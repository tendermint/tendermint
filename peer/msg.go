package peer

import (
    . "github.com/tendermint/tendermint/binary"
    "io"
)

/* Msg */

type Msg struct {
    Bytes       ByteSlice
    Hash        ByteSlice
}


/* InboundMsg */

type InboundMsg struct {
    Peer        *Peer
    Channel     *Channel
    Time        Time
    Msg
}


/* NewFilterMsg */

type NewFilterMsg struct {
    ChName          String
    Filter          interface{} // todo
}

func (m *NewFilterMsg) WriteTo(w io.Writer) (int64, error) {
    return 0, nil // TODO
}
