package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"io/ioutil"
	"os"

	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	btypes "github.com/tendermint/tendermint/cmd/barak/types"
	. "github.com/tendermint/tendermint/common"
)

var Config = struct {
	Remotes []string
	PrivKey acm.PrivKey
}{}

var (
	configFlag = cli.StringFlag{
		Name:  "config-file",
		Value: ".debora/config.json",
		Usage: "config file",
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
		configFlag,
	}
	app.Before = func(c *cli.Context) error {
		ReadConfig(c.String("config-file"))
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
		},
		cli.Command{
			Name:   "list",
			Usage:  "list processes",
			Action: cliListProcesses,
		},
	}
	app.Run(os.Args)
}

func ReadConfig(configFilePath string) {
	configJSONBytes, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		Exit(Fmt("Failed to read config file %v. %v\n", configFilePath, err))
	}
	binary.ReadJSON(&Config, configJSONBytes, &err)
	if err != nil {
		Exit(Fmt("Failed to parse config. %v", err))
	}
}

func cliGetStatus(c *cli.Context) {
	args := c.Args()
	if len(args) != 0 {
		fmt.Println("BTW, status takes no arguments.")
	}
	for _, remote := range Config.Remotes {
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
	for _, remote := range Config.Remotes {
		response, err := RunProcess(Config.PrivKey, remote, command)
		if err != nil {
			fmt.Printf("%v failure. %v\n", remote, err)
		} else {
			fmt.Printf("%v success.\n", remote)
			if response.Output != "" {
				fmt.Println("--------------------------------------------------------------------------------")
				fmt.Println(response.Output)
				fmt.Println("--------------------------------------------------------------------------------")
			} else {
				fmt.Println("(no output)")
			}
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
		Kill:  true,
	}
	for _, remote := range Config.Remotes {
		response, err := StopProcess(Config.PrivKey, remote, command)
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
	for _, remote := range Config.Remotes {
		response, err := ListProcesses(Config.PrivKey, remote, command)
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
