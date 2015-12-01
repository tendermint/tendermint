package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tmsp/types"

	"github.com/codegangsta/cli"
)

// connection is a global variable so it can be reused by the console
var conn net.Conn

func main() {
	app := cli.NewApp()
	app.Name = "cli"
	app.Usage = "cli [command] [args...]"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "address",
			Value: "tcp://127.0.0.1:8080",
			Usage: "address of application socket",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:  "console",
			Usage: "Start an interactive tmsp console for multiple commands",
			Action: func(c *cli.Context) {
				cmdConsole(app, c)
			},
		},
		{
			Name:  "append_tx",
			Usage: "Append a new tx to application",
			Action: func(c *cli.Context) {
				cmdAppendTx(c)
			},
		},
		{
			Name:  "get_hash",
			Usage: "Get application Merkle root hash",
			Action: func(c *cli.Context) {
				cmdGetHash(c)
			},
		},
		{
			Name:  "commit",
			Usage: "Commit the application state",
			Action: func(c *cli.Context) {
				cmdCommit(c)
			},
		},
		{
			Name:  "rollback",
			Usage: "Roll back the application state to the latest commit",
			Action: func(c *cli.Context) {
				cmdRollback(c)
			},
		},
	}
	app.Before = before
	app.Run(os.Args)

}

func before(c *cli.Context) error {
	if conn == nil {
		var err error
		conn, err = Connect(c.GlobalString("address"))
		if err != nil {
			Exit(err.Error())
		}
	}
	return nil
}

//--------------------------------------------------------------------------------

func cmdConsole(app *cli.App, c *cli.Context) {
	for {
		fmt.Printf("> ")
		bufReader := bufio.NewReader(os.Stdin)
		line, more, err := bufReader.ReadLine()
		if more {
			Exit("input is too long")
		} else if err != nil {
			Exit(err.Error())
		}

		args := []string{"tmsp"}
		args = append(args, strings.Split(string(line), " ")...)
		app.Run(args)
	}
}

// Append a new tx to application
func cmdAppendTx(c *cli.Context) {
	args := c.Args() // Args to AppendTx
	res, err := makeRequest(conn, types.RequestAppendTx{[]byte(args[0])})
	if err != nil {
		Exit(err.Error())
	}
	fmt.Println("Sent tx:", args[0], "response:", res)
}

// Get application Merkle root hash
func cmdGetHash(c *cli.Context) {
	res, err := makeRequest(conn, types.RequestGetHash{})
	if err != nil {
		Exit(err.Error())
	}
	fmt.Println("Got hash:", Fmt("%X", res.(types.ResponseGetHash).Hash))
}

// Commit the application state
func cmdCommit(c *cli.Context) {
	_, err := makeRequest(conn, types.RequestCommit{})
	if err != nil {
		Exit(err.Error())
	}
	fmt.Println("Committed.")
}

// Roll back the application state to the latest commit
func cmdRollback(c *cli.Context) {
	_, err := makeRequest(conn, types.RequestRollback{})
	if err != nil {
		Exit(err.Error())
	}
	fmt.Println("Rolled back.")
}

//--------------------------------------------------------------------------------

func makeRequest(conn net.Conn, req types.Request) (types.Response, error) {
	var n int
	var err error

	// Write desired request
	wire.WriteBinary(req, conn, &n, &err)
	if err != nil {
		return nil, err
	}

	// Write flush request
	wire.WriteBinary(types.RequestFlush{}, conn, &n, &err)
	if err != nil {
		return nil, err
	}

	// Read desired response
	var res types.Response
	wire.ReadBinaryPtr(&res, conn, 0, &n, &err)
	if err != nil {
		return nil, err
	}

	// Read flush response
	var resFlush types.ResponseFlush
	wire.ReadBinaryPtr(&resFlush, conn, 0, &n, &err)
	if err != nil {
		return nil, err
	}

	return res, nil
}
