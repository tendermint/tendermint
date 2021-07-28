package net

import (
	"context"
	"net"
	"strings"
	"syscall"
)

// Connect dials the given address and returns a net.Conn. The protoAddr argument should be prefixed with the protocol,
// eg. "tcp://127.0.0.1:8080" or "unix:///tmp/test.sock"
func Connect(protoAddr string) (net.Conn, error) {
	proto, address := ProtocolAndAddress(protoAddr)
	conn, err := net.Dial(proto, address)
	return conn, err
}

// ProtocolAndAddress splits an address into the protocol and address components.
// For instance, "tcp://127.0.0.1:8080" will be split into "tcp" and "127.0.0.1:8080".
// If the address has no protocol prefix, the default is "tcp".
func ProtocolAndAddress(listenAddr string) (string, string) {
	protocol, address := "tcp", listenAddr
	parts := strings.SplitN(address, "://", 2)
	if len(parts) == 2 {
		protocol, address = parts[0], parts[1]
	}
	return protocol, address
}

// GetFreePort gets a free port from the operating system.
// Ripped from https://github.com/phayes/freeport.
// BSD-licensed.
func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	// Defines a control function callback.
	// The callback will be called after the socket is created but before it is
	// 'bound'.
	// For more information, see https://pkg.go.dev/net#ListenConfig.
	controlFn := func(network, address string, c syscall.RawConn) error {
		c.Control(func(fd uintptr) {
			// Set SO_REUSEADDR on the socket.
			// SO_REUSEADDR allows a socket to be reused as soon as it is closed.
			// This is necessary because the caller of GetFreePort is likely going
			// to use the port immediately.
			syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
		})
		return nil
	}

	listenCfg := net.ListenConfig{
		Control: controlFn,
	}
	l, err := listenCfg.Listen(context.Background(), "tcp", addr.String())

	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
