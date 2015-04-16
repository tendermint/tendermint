package main

import (
	"fmt"
	"github.com/codegangsta/cli"
	"io/ioutil"
	"os"
	"strings"

	acm "github.com/tendermint/tendermint/account"
	"github.com/tendermint/tendermint/binary"
	. "github.com/tendermint/tendermint/common"
)

func main() {
	fmt.Printf("New Debora Process (PID: %d)\n", os.Getpid())
	app := cli.NewApp()
	app.Name = "debora"
	app.Usage = "summons commands to barak"
	app.Version = "0.0.1"
	app.Email = "ethan@erisindustries.com,jae@tendermint.com"
	app.Flags = []cli.Flag{}
	app.Commands = []cli.Command{
		cli.Command{
			Name:   "list",
			Usage:  "list processes",
			Action: cliListProcesses,
			Flags: []cli.Flag{
				remotesFlag,
				privKeyFlag,
			},
		},
	}
	app.Run(os.Args)
}

var (
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

func cliListProcesses(c *cli.Context) {
	remotes, privKey := ParseFlags(c)
	/*
		args := c.Args()
		if len(args) == 0 {
			log.Fatal("Must specify application name")
		}
		app := args[0]
	*/
	for _, remote := range remotes {
		response, err := ListProcesses(privKey, remote)
		if err != nil {
			fmt.Printf("%v failed. %v\n", remote, err)
		} else {
			fmt.Printf("%v processes: %v\n", remote, response.Processes)
		}
	}
}
