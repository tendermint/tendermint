package main

import (
	"fmt"
	"net"
	"os"

	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire"
	"github.com/tendermint/tmsp/types"

	"github.com/codegangsta/cli"
)

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
	app.Run(os.Args)

}

//--------------------------------------------------------------------------------

// Append a new tx to application
func cmdAppendTx(c *cli.Context) {
	args := c.Args() // Args to AppendTx
	conn, err := Connect(c.GlobalString("address"))
	if err != nil {
		Exit(err.Error())
	}
	res, err := write(conn, types.RequestAppendTx{[]byte(args[0])})
	if err != nil {
		Exit(err.Error())
	}
	fmt.Println("Sent tx:", args[0], "response:", res)
}

// Get application Merkle root hash
func cmdGetHash(c *cli.Context) {
	conn, err := Connect(c.GlobalString("address"))
	if err != nil {
		Exit(err.Error())
	}
	res, err := write(conn, types.RequestGetHash{})
	if err != nil {
		Exit(err.Error())
	}
	fmt.Println("Got hash:", Fmt("%X", res.(types.ResponseGetHash).Hash))
}

// Commit the application state
func cmdCommit(c *cli.Context) {
	conn, err := Connect(c.GlobalString("address"))
	if err != nil {
		Exit(err.Error())
	}
	_, err = write(conn, types.RequestCommit{})
	if err != nil {
		Exit(err.Error())
	}
	fmt.Println("Committed.")
}

// Roll back the application state to the latest commit
func cmdRollback(c *cli.Context) {
	conn, err := Connect(c.GlobalString("address"))
	if err != nil {
		Exit(err.Error())
	}
	_, err = write(conn, types.RequestRollback{})
	if err != nil {
		Exit(err.Error())
	}
	fmt.Println("Rolled back.")
}

//--------------------------------------------------------------------------------

func write(conn net.Conn, req types.Request) (types.Response, error) {
	var n int64
	var err error
	wire.WriteBinary(req, conn, &n, &err)
	if err != nil {
		return nil, err
	}
	var res types.Response
	wire.ReadBinaryPtr(&res, conn, &n, &err)
	return res, err
}
