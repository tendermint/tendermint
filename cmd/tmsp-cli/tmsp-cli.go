package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/codegangsta/cli"
	. "github.com/tendermint/go-common"
	"github.com/tendermint/go-wire/expr"
	"github.com/tendermint/tmsp/client"
	"github.com/tendermint/tmsp/types"
)

// client is a global variable so it can be reused by the console
var client tmspcli.Client

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
		cli.StringFlag{
			Name:  "tmsp",
			Value: "socket",
			Usage: "socket or grpc",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:  "batch",
			Usage: "Run a batch of tmsp commands against an application",
			Action: func(c *cli.Context) error {
				return cmdBatch(app, c)
			},
		},
		{
			Name:  "console",
			Usage: "Start an interactive tmsp console for multiple commands",
			Action: func(c *cli.Context) error {
				return cmdConsole(app, c)
			},
		},
		{
			Name:  "echo",
			Usage: "Have the application echo a message",
			Action: func(c *cli.Context) error {
				return cmdEcho(c)
			},
		},
		{
			Name:  "info",
			Usage: "Get some info about the application",
			Action: func(c *cli.Context) error {
				return cmdInfo(c)
			},
		},
		{
			Name:  "set_option",
			Usage: "Set an option on the application",
			Action: func(c *cli.Context) error {
				return cmdSetOption(c)
			},
		},
		{
			Name:  "append_tx",
			Usage: "Append a new tx to application",
			Action: func(c *cli.Context) error {
				return cmdAppendTx(c)
			},
		},
		{
			Name:  "check_tx",
			Usage: "Validate a tx",
			Action: func(c *cli.Context) error {
				return cmdCheckTx(c)
			},
		},
		{
			Name:  "commit",
			Usage: "Commit the application state and return the Merkle root hash",
			Action: func(c *cli.Context) error {
				return cmdCommit(c)
			},
		},
		{
			Name:  "query",
			Usage: "Query application state",
			Action: func(c *cli.Context) error {
				return cmdQuery(c)
			},
		},
	}
	app.Before = before
	err := app.Run(os.Args)
	if err != nil {
		Exit(err.Error())
	}

}

func before(c *cli.Context) error {
	if client == nil {
		var err error
		client, err = tmspcli.NewClient(c.GlobalString("address"), c.GlobalString("tmsp"), false)
		if err != nil {
			Exit(err.Error())
		}
	}
	return nil
}

//--------------------------------------------------------------------------------

func cmdBatch(app *cli.App, c *cli.Context) error {
	bufReader := bufio.NewReader(os.Stdin)
	for {
		line, more, err := bufReader.ReadLine()
		if more {
			return errors.New("Input line is too long")
		} else if err == io.EOF {
			break
		} else if len(line) == 0 {
			continue
		} else if err != nil {
			return err
		}
		args := []string{"tmsp"}
		args = append(args, strings.Split(string(line), " ")...)
		app.Run(args)
	}
	return nil
}

func cmdConsole(app *cli.App, c *cli.Context) error {
	for {
		fmt.Printf("\n> ")
		bufReader := bufio.NewReader(os.Stdin)
		line, more, err := bufReader.ReadLine()
		if more {
			return errors.New("Input is too long")
		} else if err != nil {
			return err
		}

		args := []string{"tmsp"}
		args = append(args, strings.Split(string(line), " ")...)
		if err := app.Run(args); err != nil {
			return err
		}
	}
	return nil
}

// Have the application echo a message
func cmdEcho(c *cli.Context) error {
	args := c.Args()
	if len(args) != 1 {
		return errors.New("Command echo takes 1 argument")
	}
	res := client.EchoSync(args[0])
	printResponse(res, string(res.Data), false)
	return nil
}

// Get some info from the application
func cmdInfo(c *cli.Context) error {
	res := client.InfoSync()
	printResponse(res, string(res.Data), false)
	return nil
}

// Set an option on the application
func cmdSetOption(c *cli.Context) error {
	args := c.Args()
	if len(args) != 2 {
		return errors.New("Command set_option takes 2 arguments (key, value)")
	}
	res := client.SetOptionSync(args[0], args[1])
	printResponse(res, Fmt("%s=%s", args[0], args[1]), false)
	return nil
}

// Append a new tx to application
func cmdAppendTx(c *cli.Context) error {
	args := c.Args()
	if len(args) != 1 {
		return errors.New("Command append_tx takes 1 argument")
	}
	txExprString := c.Args()[0]
	txBytes, err := expr.Compile(txExprString)
	if err != nil {
		return err
	}

	res := client.AppendTxSync(txBytes)
	printResponse(res, string(res.Data), true)
	return nil
}

// Validate a tx
func cmdCheckTx(c *cli.Context) error {
	args := c.Args()
	if len(args) != 1 {
		return errors.New("Command check_tx takes 1 argument")
	}
	txExprString := c.Args()[0]
	txBytes, err := expr.Compile(txExprString)
	if err != nil {
		return err
	}

	res := client.CheckTxSync(txBytes)
	printResponse(res, string(res.Data), true)
	return nil
}

// Get application Merkle root hash
func cmdCommit(c *cli.Context) error {
	res := client.CommitSync()
	printResponse(res, Fmt("%X", res.Data), false)
	return nil
}

// Query application state
func cmdQuery(c *cli.Context) error {
	args := c.Args()
	if len(args) != 1 {
		return errors.New("Command query takes 1 argument")
	}
	queryExprString := args[0]
	queryBytes, err := expr.Compile(queryExprString)
	if err != nil {
		return err
	}

	res := client.QuerySync(queryBytes)
	printResponse(res, string(res.Data), true)
	return nil
}

//--------------------------------------------------------------------------------

func printResponse(res types.Result, s string, printCode bool) {
	if printCode {
		fmt.Printf("-> code: %s\n", res.Code.String())
	}
	/*if res.Error != "" {
		fmt.Printf("-> error: %s\n", res.Error)
	}*/
	if s != "" {
		fmt.Printf("-> data: {%s}\n", s)
	}
	if res.Log != "" {
		fmt.Printf("-> log: %s\n", res.Log)
	}

}
