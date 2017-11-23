package main

import (
	"io/ioutil"
	"log"
	"net"
)

const addr = "0.0.0.0:8080"

func main() {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		// log.Fatal(err)
		return
	}

	log.Printf("running server at %q", addr)
	i := uint64(0)
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("conn err:: %d: %v", i, err)
		} else {
			go handleConnection(conn)
		}
	}
}

func handleConnection(conn net.Conn) {
	data, err := ioutil.ReadAll(conn)
	log.Printf("err: %v data: %d\n", err, len(data))
}
