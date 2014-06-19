package peer

import (
    "testing"
)

func TestLocalConnection(t *testing.T) {

    c1 := NewClient("tcp", ":8080")
    c2 := NewClient("tcp", ":8081")

    c1.ConnectTo(c2.LocalAddress())

    c1.Stop()
    c2.Stop()
}
