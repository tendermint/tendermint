package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"io/ioutil"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"

	acm "github.com/eris-ltd/tendermint/account"
	btypes "github.com/eris-ltd/tendermint/cmd/barak/types"
	. "github.com/eris-ltd/tendermint/common"
	cfg "github.com/eris-ltd/tendermint/config"
	"github.com/eris-ltd/tendermint/wire"
)

func remoteNick(remote string) string {
	u, err := url.Parse(remote)
	if err != nil {
		return regexp.MustCompile(`[[:^alnum:]]`).ReplaceAllString(remote, "_")
	} else {
		return regexp.MustCompile(`[[:^alnum:]]`).ReplaceAllString(u.Host, "_")
	}
}

var Config = struct {
	Remotes []string
	PrivKey acm.PrivKey
}{}

func main() {
	fmt.Printf("New Debora Process (PID: %d)\n", os.Getpid())

	// Apply bare tendermint/* configuration.
	config := cfg.NewMapConfig(nil)
	config.Set("log_level", "notice")
	cfg.ApplyConfig(config)

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
		labelFlag = cli.StringFlag{
			Name:  "label",
			Value: "_",
			Usage: "label of the process, or _ by default",
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
			Action: cliStartProcess,
			Flags: []cli.Flag{
				labelFlag,
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
			Name:   "open",
			Usage:  "open barak listener",
			Action: cliOpenListener,
		},
		cli.Command{
			Name:   "close",
			Usage:  "close barka listener",
			Action: cliCloseListener,
		},
		cli.Command{
			Name:   "download",
			Usage:  "download file <remote-path> <local-path-prefix>",
			Action: cliDownloadFile,
		},
		cli.Command{
			Name:   "quit",
			Usage:  "quit barak",
			Action: cliQuit,
		},
	}
	app.Run(os.Args)
}

func ReadConfig(configFilePath string) {
	configJSONBytes, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		Exit(Fmt("Failed to read config file %v. %v\n", configFilePath, err))
	}
	wire.ReadJSON(&Config, configJSONBytes, &err)
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
	failed := 0
	for _, remote := range Config.Remotes {
		wg.Add(1)
		go func(remote string) {
			defer wg.Done()
			response, err := GetStatus(remote)
			if err != nil {
				failed++
				fmt.Printf("%v failure. %v\n", remote, err)
			} else {
				fmt.Printf("%v success. %v\n", remote, response)
			}
		}(remote)
	}
	wg.Wait()
	if 0 < failed {
		os.Exit(1)
	}
}

func cliStartProcess(c *cli.Context) {
	args := c.Args()
	if len(args) < 1 {
		Exit("Must specify <execPath> <args...>")
	}
	execPath := args[0]
	args = args[1:]
	command := btypes.CommandStartProcess{
		Wait:     !c.Bool("bg"),
		Label:    c.String("label"),
		ExecPath: execPath,
		Args:     args,
		Input:    c.String("input"),
	}
	wg := sync.WaitGroup{}
	failed := 0
	for _, remote := range Config.Remotes {
		wg.Add(1)
		go func(remote string) {
			defer wg.Done()
			response, err := StartProcess(Config.PrivKey, remote, command)
			if err != nil {
				failed++
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
	if 0 < failed {
		os.Exit(1)
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
	wg := sync.WaitGroup{}
	failed := 0
	for _, remote := range Config.Remotes {
		wg.Add(1)
		go func(remote string) {
			defer wg.Done()
			response, err := StopProcess(Config.PrivKey, remote, command)
			if err != nil {
				failed++
				fmt.Printf("%v failure. %v\n", remote, err)
			} else {
				fmt.Printf("%v success. %v\n", remote, response)
			}
		}(remote)
	}
	wg.Wait()
	if 0 < failed {
		os.Exit(1)
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
	wg := sync.WaitGroup{}
	failed := 0
	for _, remote := range Config.Remotes {
		wg.Add(1)
		go func(remote string) {
			defer wg.Done()
			response, err := ListProcesses(Config.PrivKey, remote, command)
			if err != nil {
				failed++
				fmt.Printf("%v failure. %v\n", Blue(remote), Red(err))
			} else {
				fmt.Printf("%v processes:\n", Blue(remote))
				for _, proc := range response.Processes {
					fmt.Printf("  \"%v\" => `%v  %v` (%v)\n", Yellow(proc.Label), proc.ExecPath, strings.Join(proc.Args, ","), proc.Pid)
					fmt.Printf("     started at %v", proc.StartTime.String())
					if proc.EndTime.IsZero() {
						fmt.Printf(", running still\n")
					} else {
						endTimeStr := proc.EndTime.String()
						fmt.Printf(", stopped at %v\n", Yellow(endTimeStr))
					}
				}
			}
		}(remote)
	}
	wg.Wait()
	if 0 < failed {
		os.Exit(1)
	}
}

func cliOpenListener(c *cli.Context) {
	args := c.Args()
	if len(args) < 1 {
		Exit("Must specify <listenAddr e.g. [::]:46661>")
	}
	listenAddr := args[0]
	command := btypes.CommandOpenListener{
		Addr: listenAddr,
	}
	wg := sync.WaitGroup{}
	failed := 0
	for _, remote := range Config.Remotes {
		wg.Add(1)
		go func(remote string) {
			defer wg.Done()
			response, err := OpenListener(Config.PrivKey, remote, command)
			if err != nil {
				failed++
				fmt.Printf("%v failure. %v\n", remote, err)
			} else {
				fmt.Printf("%v opened %v.\n", remote, response.Addr)
			}
		}(remote)
	}
	wg.Wait()
	if 0 < failed {
		os.Exit(1)
	}
}

func cliCloseListener(c *cli.Context) {
	args := c.Args()
	if len(args) == 0 {
		Exit("Must specify listenAddr to stop")
	}
	listenAddr := args[0]
	command := btypes.CommandCloseListener{
		Addr: listenAddr,
	}
	wg := sync.WaitGroup{}
	failed := 0
	for _, remote := range Config.Remotes {
		wg.Add(1)
		go func(remote string) {
			defer wg.Done()
			response, err := CloseListener(Config.PrivKey, remote, command)
			if err != nil {
				failed++
				fmt.Printf("%v failure. %v\n", remote, err)
			} else {
				fmt.Printf("%v success. %v\n", remote, response)
			}
		}(remote)
	}
	wg.Wait()
	if 0 < failed {
		os.Exit(1)
	}
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
	failed := 0
	for _, remote := range Config.Remotes {
		wg.Add(1)
		go func(remote string, localPath string) {
			defer wg.Done()
			n, err := DownloadFile(Config.PrivKey, remote, command, localPath)
			if err != nil {
				failed++
				fmt.Printf("%v failure. %v\n", remote, err)
			} else {
				fmt.Printf("%v success. Wrote %v bytes to %v\n", remote, n, localPath)
			}
		}(remote, Fmt("%v_%v", localPathPrefix, remoteNick(remote)))
	}
	wg.Wait()
	if 0 < failed {
		os.Exit(1)
	}
}

func cliQuit(c *cli.Context) {
	command := btypes.CommandQuit{}
	wg := sync.WaitGroup{}
	failed := 0
	for _, remote := range Config.Remotes {
		wg.Add(1)
		go func(remote string) {
			defer wg.Done()
			response, err := Quit(Config.PrivKey, remote, command)
			if err != nil {
				failed++
				fmt.Printf("%v failure. %v\n", remote, err)
			} else {
				fmt.Printf("%v success. %v\n", remote, response)
			}
		}(remote)
	}
	wg.Wait()
	if 0 < failed {
		os.Exit(1)
	}
}
