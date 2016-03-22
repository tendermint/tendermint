package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"

	"github.com/codegangsta/cli"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire/expr"
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
			Name:  "commit",
			Usage: "Commit the application state and return the Merkle root hash",
			Action: func(c *cli.Context) {
				cmdCommit(c)
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
	printResponse(res, string(res.Data))
}

// Get some info from the application
func cmdInfo(c *cli.Context) {
	res, err := makeRequest(conn, types.RequestInfo())
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	printResponse(res, string(res.Data))
}

// Set an option on the application
func cmdSetOption(c *cli.Context) {
	args := c.Args()
	if len(args) != 2 {
		fmt.Println("set_option takes 2 arguments (key, value)")
		return
	}
	res, err := makeRequest(conn, types.RequestSetOption(args[0], args[1]))
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	printResponse(res, Fmt("%s=%s", args[0], args[1]))
}

// Append a new tx to application
func cmdAppendTx(c *cli.Context) {
	args := c.Args()
	if len(args) != 1 {
		fmt.Println("append_tx takes 1 argument")
		return
	}
	txExprString := c.Args()[0]
	txBytes, err := expr.Compile(txExprString)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	res, err := makeRequest(conn, types.RequestAppendTx(txBytes))
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	printResponse(res, string(res.Data))
}

// Validate a tx
func cmdCheckTx(c *cli.Context) {
	args := c.Args()
	if len(args) != 1 {
		fmt.Println("check_tx takes 1 argument")
		return
	}
	txExprString := c.Args()[0]
	txBytes, err := expr.Compile(txExprString)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	res, err := makeRequest(conn, types.RequestCheckTx(txBytes))
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	printResponse(res, string(res.Data))
}

// Get application Merkle root hash
func cmdCommit(c *cli.Context) {
	res, err := makeRequest(conn, types.RequestCommit())
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	printResponse(res, Fmt("%X", res.Data))
}

// Query application state
func cmdQuery(c *cli.Context) {
	args := c.Args()
	if len(args) != 1 {
		fmt.Println("query takes 1 argument")
		return
	}
	queryExprString := args[0]
	queryBytes, err := expr.Compile(queryExprString)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	res, err := makeRequest(conn, types.RequestQuery(queryBytes))
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	printResponse(res, string(res.Data))
}

//--------------------------------------------------------------------------------

func printResponse(res *types.Response, s string) {
	switch res.Type {
	case types.MessageType_AppendTx, types.MessageType_CheckTx, types.MessageType_Query:
		fmt.Printf("-> code: %s\n", res.Code.String())
	}
	if res.Error != "" {
		fmt.Printf("-> error: %s\n", res.Error)
	}
	if s != "" {
		fmt.Printf("-> data: {%s}\n", s)
	}
	if res.Log != "" {
		fmt.Printf("-> log: %s\n", res.Log)
	}

}

func responseString(res *types.Response) string {
	return Fmt("type: %v\tdata: %v\tcode: %v", res.Type, res.Data, res.Code)
}

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
	if resFlush.Type != types.MessageType_Flush {
		return nil, errors.New(Fmt("Expected types.MessageType_Flush but got %v instead", resFlush.Type))
	}

	return res, nil
}
