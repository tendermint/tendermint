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
	waitFlag = cli.BoolFlag{
		Name:  "wait",
		Usage: "whether to wait for termination",
	}
	inputFlag = cli.StringFlag{
		Name:  "input",
		Value: "",
		Usage: "input to the program (e.g. stdin)",
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
			Name:   "status",
			Usage:  "shows remote status",
			Action: cliGetStatus,
		},
		cli.Command{
			Name:   "run",
			Usage:  "run process",
			Action: cliRunProcess,
			Flags: []cli.Flag{
				waitFlag,
				inputFlag,
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
		fmt.Printf("Failed to read privkey from file %v. %v", privkeyFile, err)
		return remotes, nil
	}
	binary.ReadJSON(&privKey, privkeyJSONBytes, &err)
	if err != nil {
		Exit(Fmt("Failed to parse privkey. %v", err))
	}
	return remotes, privKey
}

func cliGetStatus(c *cli.Context) {
	args := c.Args()
	if len(args) != 0 {
		fmt.Println("BTW, status takes no arguments.")
	}
	for _, remote := range remotes {
		response, err := GetStatus(remote)
		if err != nil {
			fmt.Printf("%v failure. %v\n", remote, err)
		} else {
			fmt.Printf("%v success. %v\n", remote, response)
		}
	}
}

func cliRunProcess(c *cli.Context) {
	args := c.Args()
	if len(args) < 2 {
		Exit("Must specify <label> <execPath> <args...>")
	}
	label := args[0]
	execPath := args[1]
	args = args[2:]
	command := btypes.CommandRunProcess{
		Wait:     c.Bool("wait"),
		Label:    label,
		ExecPath: execPath,
		Args:     args,
		Input:    c.String("input"),
	}
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
	args := c.Args()
	if len(args) == 0 {
		Exit("Must specify label to stop")
	}
	label := args[0]
	command := btypes.CommandStopProcess{
		Label: label,
	}
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
			fmt.Printf("%v processes:\n", remote)
			for _, proc := range response.Processes {
				fmt.Printf("  \"%v\" => `%v` (%v)  start:%v end:%v output:%v\n",
					proc.Label, proc.ExecPath, proc.Pid,
					proc.StartTime, proc.EndTime, proc.OutputPath)
			}
		}
	}
}
