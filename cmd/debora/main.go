package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"io/ioutil"
	"os"
	"strings"

	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	btypes "github.com/tendermint/tendermint/cmd/barak/types"
	. "github.com/tendermint/tendermint/common"
)

var (
	remotes     []string
	privKey     acm.PrivKey
	remotesFlag = cli.StringFlag{
		Name:  "remotes",
		Value: "http://127.0.0.1:8082",
		Usage: "comma separated list of remote baraks",
	}
	privKeyFlag = cli.StringFlag{
		Name:  "privkey-file",
		Value: "privkey",
		Usage: "file containing private key json",
	}
)

func main() {
	fmt.Printf("New Debora Process (PID: %d)\n", os.Getpid())
	app := cli.NewApp()
	app.Name = "debora"
	app.Usage = "summons commands to barak"
	app.Version = "0.0.1"
	app.Email = "ethan@erisindustries.com,jae@tendermint.com"
	app.Flags = []cli.Flag{
		remotesFlag,
		privKeyFlag,
	}
	app.Before = func(c *cli.Context) error {
		remotes, privKey = ParseFlags(c)
		return nil
	}
	app.Commands = []cli.Command{
		cli.Command{
			Name:   "run",
			Usage:  "run process",
			Action: cliRunProcess,
			Flags:  []cli.Flag{
			//remotesFlag,
			//privKeyFlag,
			},
		},
		cli.Command{
			Name:   "stop",
			Usage:  "stop process",
			Action: cliStopProcess,
			Flags:  []cli.Flag{
			//remotesFlag,
			//privKeyFlag,
			},
		},
		cli.Command{
			Name:   "list",
			Usage:  "list processes",
			Action: cliListProcesses,
			Flags:  []cli.Flag{
			//remotesFlag,
			//privKeyFlag,
			},
		},
	}
	app.Run(os.Args)
}

func ParseFlags(c *cli.Context) (remotes []string, privKey acm.PrivKey) {
	remotesStr := c.String("remotes")
	remotes = strings.Split(remotesStr, ",")
	privkeyFile := c.String("privkey-file")
	privkeyJSONBytes, err := ioutil.ReadFile(privkeyFile)
	if err != nil {
		Exit(Fmt("Failed to read privkey from file %v. %v", privkeyFile, err))
	}
	binary.ReadJSON(&privKey, privkeyJSONBytes, &err)
	if err != nil {
		Exit(Fmt("Failed to parse privkey. %v", err))
	}
	return remotes, privKey
}

func cliRunProcess(c *cli.Context) {
	/*
		args := c.Args()
		if len(args) == 0 {
			log.Fatal("Must specify application name")
		}
		app := args[0]
	*/
	command := btypes.CommandRunProcess{}
	for _, remote := range remotes {
		response, err := RunProcess(privKey, remote, command)
		if err != nil {
			fmt.Printf("%v failure. %v\n", remote, err)
		} else {
			fmt.Printf("%v success. %v\n", remote, response)
		}
	}
}

func cliStopProcess(c *cli.Context) {
	/*
		args := c.Args()
		if len(args) == 0 {
			log.Fatal("Must specify application name")
		}
		app := args[0]
	*/
	command := btypes.CommandStopProcess{}
	for _, remote := range remotes {
		response, err := StopProcess(privKey, remote, command)
		if err != nil {
			fmt.Printf("%v failure. %v\n", remote, err)
		} else {
			fmt.Printf("%v success. %v\n", remote, response)
		}
	}
}

func cliListProcesses(c *cli.Context) {
	/*
		args := c.Args()
		if len(args) == 0 {
			log.Fatal("Must specify application name")
		}
		app := args[0]
	*/
	command := btypes.CommandListProcesses{}
	for _, remote := range remotes {
		response, err := ListProcesses(privKey, remote, command)
		if err != nil {
			fmt.Printf("%v failure. %v\n", remote, err)
		} else {
			fmt.Printf("%v success: %v\n", remote, response)
		}
	}
}
