package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/codegangsta/cli"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/tmsp/types"
)

// connection is a global variable so it can be reused by the console
var conn net.Conn

func main() {
	app := cli.NewApp()
	app.Name = "tmsp-cli"
	app.Usage = "tmsp-cli [command] [args...]"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "address",
			Value: "tcp://127.0.0.1:46658",
			Usage: "address of application socket",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:  "batch",
			Usage: "Run a batch of tmsp commands against an application",
			Action: func(c *cli.Context) {
				cmdBatch(app, c)
			},
		},
		{
			Name:  "console",
			Usage: "Start an interactive tmsp console for multiple commands",
			Action: func(c *cli.Context) {
				cmdConsole(app, c)
			},
		},
		{
			Name:  "echo",
			Usage: "Have the application echo a message",
			Action: func(c *cli.Context) {
				cmdEcho(c)
			},
		},
		{
			Name:  "info",
			Usage: "Get some info about the application",
			Action: func(c *cli.Context) {
				cmdInfo(c)
			},
		},
		{
			Name:  "set_option",
			Usage: "Set an option on the application",
			Action: func(c *cli.Context) {
				cmdSetOption(c)
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
			Name:  "check_tx",
			Usage: "Validate a tx",
			Action: func(c *cli.Context) {
				cmdCheckTx(c)
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
			Name:  "query",
			Usage: "Query application state",
			Action: func(c *cli.Context) {
				cmdQuery(c)
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

func cmdBatch(app *cli.App, c *cli.Context) {
	bufReader := bufio.NewReader(os.Stdin)
	for {
		line, more, err := bufReader.ReadLine()
		if more {
			fmt.Println("input line is too long")
			return
		} else if err == io.EOF {
			break
		} else if len(line) == 0 {
			continue
		} else if err != nil {
			fmt.Println(err.Error())
			return
		}
		args := []string{"tmsp"}
		args = append(args, strings.Split(string(line), " ")...)
		app.Run(args)
	}
}

func cmdConsole(app *cli.App, c *cli.Context) {
	for {
		fmt.Printf("\n> ")
		bufReader := bufio.NewReader(os.Stdin)
		line, more, err := bufReader.ReadLine()
		if more {
			fmt.Println("input is too long")
			return
		} else if err != nil {
			fmt.Println(err.Error())
			return
		}

		args := []string{"tmsp"}
		args = append(args, strings.Split(string(line), " ")...)
		app.Run(args)
	}
}

// Have the application echo a message
func cmdEcho(c *cli.Context) {
	args := c.Args()
	if len(args) != 1 {
		fmt.Println("echo takes 1 argument")
		return
	}
	res, err := makeRequest(conn, types.RequestEcho(args[0]))
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("->", res)
}

// Get some info from the application
func cmdInfo(c *cli.Context) {
	res, err := makeRequest(conn, types.RequestInfo())
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("->", res)
}

// Set an option on the application
func cmdSetOption(c *cli.Context) {
	args := c.Args()
	if len(args) != 2 {
		fmt.Println("set_option takes 2 arguments (key, value)")
		return
	}
	_, err := makeRequest(conn, types.RequestSetOption(args[0], args[1]))
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("->", Fmt("%s=%s", args[0], args[1]))
}

// Append a new tx to application
func cmdAppendTx(c *cli.Context) {
	args := c.Args()
	if len(args) != 1 {
		fmt.Println("append_tx takes 1 argument")
		return
	}
	txString := args[0]
	tx := []byte(txString)
	if len(txString) > 2 && strings.HasPrefix(txString, "0x") {
		var err error
		tx, err = hex.DecodeString(txString[2:])
		if err != nil {
			fmt.Println(err.Error())
			return
		}
	}

	res, err := makeRequest(conn, types.RequestAppendTx(tx))
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("->", res)
}

// Validate a tx
func cmdCheckTx(c *cli.Context) {
	args := c.Args()
	if len(args) != 1 {
		fmt.Println("append_tx takes 1 argument")
		return
	}
	txString := args[0]
	tx := []byte(txString)
	if len(txString) > 2 && strings.HasPrefix(txString, "0x") {
		var err error
		tx, err = hex.DecodeString(txString[2:])
		if err != nil {
			fmt.Println(err.Error())
			return
		}
	}

	res, err := makeRequest(conn, types.RequestCheckTx(tx))
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("->", res)
}

// Get application Merkle root hash
func cmdGetHash(c *cli.Context) {
	res, err := makeRequest(conn, types.RequestGetHash())
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Printf("%X\n", res.Data)
}

// Query application state
func cmdQuery(c *cli.Context) {
	args := c.Args()
	if len(args) != 1 {
		fmt.Println("append_tx takes 1 argument")
		return
	}
	queryString := args[0]
	query := []byte(queryString)
	if len(queryString) > 2 && strings.HasPrefix(queryString, "0x") {
		var err error
		query, err = hex.DecodeString(queryString[2:])
		if err != nil {
			fmt.Println(err.Error())
			return
		}
	}

	res, err := makeRequest(conn, types.RequestQuery(query))
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println("->", res)
}

//--------------------------------------------------------------------------------

func makeRequest(conn net.Conn, req *types.Request) (*types.Response, error) {

	// Write desired request
	err := types.WriteMessage(req, conn)
	if err != nil {
		return nil, err
	}

	// Write flush request
	err = types.WriteMessage(types.RequestFlush(), conn)
	if err != nil {
		return nil, err
	}

	// Read desired response
	var res = &types.Response{}
	err = types.ReadMessage(conn, res)
	if err != nil {
		return nil, err
	}

	// Read flush response
	var resFlush = &types.Response{}
	err = types.ReadMessage(conn, resFlush)
	if err != nil {
		return nil, err
	}
	if resFlush.Type != types.ResponseTypeFlush {
		return nil, errors.New(Fmt("Expected types.ResponseTypesFlush but got %v instead", resFlush.Type))
	}

	return res, nil
}
