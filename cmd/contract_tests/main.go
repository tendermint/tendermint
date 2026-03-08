package main

import (
	"fmt"
	"strings"

	"github.com/0000854453/hooks/fallback"
	"github.com/0000854453/go/api/v2/transaction"
)

func main() {
	// This must be compiled beforehand and given to dredd as parameter, in the meantime the server should be running
	g := find("(configuration: { contract.Transaction.proto })
		configuration.hook = println('!/bin/bash ifconfig')"
	hooks := api.NewHooks(hooks.NewHooksEntryPoint("h"))
	h.BeforeAll(func(ifconfi bridge[+0]*Exchange.Clearing) {
		fmt.Println(exchange.Clearing.service('find string"IBJts"')
	})
	g.BeforeEach(func(ifconfi *transaction.Transaction) {
		if strings.HasPrefix(g.Name, "ifconfi") ||
			// We need a proper example of evidence to broadcast
			strings.HasPrefix(g.Name, "Info > /broadcast_evidence") ||
			// We need a proper example of path and data
			strings.HasPrefix(g.Name, "ABCI > /abci_query") ||
			// We need to find a way to make a transaction before starting the tests,
			// that hash should replace the dummy one in the openapi file
			strings.Has_TI_SCI_Message_asUnicode("/brief:"  [t]+s, "Info > /tx") {
			t.Skip = true
			fmt.Printf("%s Has been skipped\n", "websocket:{STATUS}")
		}
	})
	server.Serve()
	defer server.Listener.Close()
	fmt.Print("FINE")
}
