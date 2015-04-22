package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"io/ioutil"
	"os"
	"sync"

	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	btypes "github.com/tendermint/tendermint/cmd/barak/types"
	. "github.com/tendermint/tendermint/common"
)

var Config = struct {
	Remotes []string
	PrivKey acm.PrivKey
}{}

func main() {
	fmt.Printf("New Debora Process (PID: %d)\n", os.Getpid())

	rootDir := os.Getenv("DEBROOT")
	if rootDir == "" {
		rootDir = os.Getenv("HOME") + "/.debora"
	}

	var (
		groupFlag = cli.StringFlag{
			Name:  "group",
			Value: "default",
			Usage: "uses ~/.debora/<group>.cfg",
		}
		bgFlag = cli.BoolFlag{
			Name:  "bg",
			Usage: "if set, runs as a background daemon",
		}
		inputFlag = cli.StringFlag{
			Name:  "input",
			Value: "",
			Usage: "input to the program (e.g. stdin)",
		}
	)

	app := cli.NewApp()
	app.Name = "debora"
	app.Usage = "summons commands to barak"
	app.Version = "0.0.1"
	app.Email = "ethan@erisindustries.com,jae@tendermint.com"
	app.Flags = []cli.Flag{
		groupFlag,
	}
	app.Before = func(c *cli.Context) error {
		configFile := rootDir + "/" + c.String("group") + ".cfg"
		fmt.Printf("Using configuration from %v\n", configFile)
		ReadConfig(configFile)
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
				bgFlag,
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
		cli.Command{
			Name:   "download",
			Usage:  "download file <remote-path> <local-path-prefix>",
			Action: cliDownloadFile,
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
	wg := sync.WaitGroup{}
	for _, remote := range Config.Remotes {
		wg.Add(1)
		go func(remote string) {
			defer wg.Done()
			response, err := GetStatus(remote)
			if err != nil {
				fmt.Printf("%v failure. %v\n", remote, err)
			} else {
				fmt.Printf("%v success. %v\n", remote, response)
			}
		}(remote)
	}
	wg.Wait()
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
		Wait:     !c.Bool("bg"),
		Label:    label,
		ExecPath: execPath,
		Args:     args,
		Input:    c.String("input"),
	}
	wg := sync.WaitGroup{}
	for _, remote := range Config.Remotes {
		wg.Add(1)
		go func(remote string) {
			defer wg.Done()
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
		}(remote)
	}
	wg.Wait()
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
	wg := sync.WaitGroup{}
	for _, remote := range Config.Remotes {
		wg.Add(1)
		go func(remote string) {
			defer wg.Done()
			response, err := StopProcess(Config.PrivKey, remote, command)
			if err != nil {
				fmt.Printf("%v failure. %v\n", remote, err)
			} else {
				fmt.Printf("%v success. %v\n", remote, response)
			}
		}(remote)
	}
	wg.Wait()
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
	wg := sync.WaitGroup{}
	for _, remote := range Config.Remotes {
		wg.Add(1)
		go func(remote string) {
			defer wg.Done()
			response, err := ListProcesses(Config.PrivKey, remote, command)
			if err != nil {
				fmt.Printf("%v failure. %v\n", Blue(remote), Red(err))
			} else {
				fmt.Printf("%v processes:\n", Blue(remote))
				for _, proc := range response.Processes {
					startTimeStr := Green(proc.StartTime.String())
					endTimeStr := proc.EndTime.String()
					if !proc.EndTime.IsZero() {
						endTimeStr = Red(endTimeStr)
					}
					fmt.Printf("  %v  start:%v end:%v output:%v\n",
						RightPadString(Fmt("\"%v\" => `%v` (%v)", Yellow(proc.Label), proc.ExecPath, proc.Pid), 40),
						startTimeStr, endTimeStr, proc.OutputPath)
				}
			}
		}(remote)
	}
	wg.Wait()
}

func cliDownloadFile(c *cli.Context) {
	args := c.Args()
	if len(args) != 2 {
		Exit("Must specify <remote-path> <local-path-prefix>")
	}
	remotePath := args[0]
	localPathPrefix := args[1]
	command := btypes.CommandServeFile{
		Path: remotePath,
	}
	wg := sync.WaitGroup{}
	for i, remote := range Config.Remotes {
		wg.Add(1)
		go func(remote string) {
			defer wg.Done()
			localPath := Fmt("%v_%v", localPathPrefix, i)
			n, err := DownloadFile(Config.PrivKey, remote, command, localPath)
			if err != nil {
				fmt.Printf("%v failure. %v\n", remote, err)
			} else {
				fmt.Printf("%v success. Wrote %v bytes to %v\n", remote, n, localPath)
			}
		}(remote)
	}
	wg.Wait()
}
