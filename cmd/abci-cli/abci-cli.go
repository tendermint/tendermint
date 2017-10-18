package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	abcicli "github.com/tendermint/abci/client"
	"github.com/tendermint/abci/types"
	//"github.com/tendermint/abci/version"
	"github.com/tendermint/tmlibs/log"

	"github.com/spf13/cobra"
)

// Structure for data passed to print response.
type response struct {
	// generic abci response
	Data []byte
	Code types.CodeType
	Log  string

	Query *queryResponse
}

type queryResponse struct {
	Key    []byte
	Value  []byte
	Height uint64
	Proof  []byte
}

// client is a global variable so it can be reused by the console
var client abcicli.Client

var logger log.Logger

// flags
var (
	address string
	abci    string
	verbose bool

	path   string
	height int
	prove  bool
)

var RootCmd = &cobra.Command{
	Use:   "abci-cli",
	Short: "",
	Long:  "",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if logger == nil {
			logger = log.NewFilter(log.NewTMLogger(log.NewSyncWriter(os.Stdout)), log.AllowError())
		}
		if client == nil {
			var err error
			client, err = abcicli.NewClient(address, abci, false)
			if err != nil {
				logger.Error(err.Error())
				os.Exit(1)
			}
			client.SetLogger(logger.With("module", "abci-client"))
			if _, err := client.Start(); err != nil {
				return err
			}
		}
		return nil
	},
}

func Execute() {
	addGlobalFlags()
	addCommands()
	RootCmd.Execute() //err?
}

func addGlobalFlags() {
	RootCmd.PersistentFlags().StringVarP(&address, "address", "", "tcp://127.0.0.1:46658", "address of application socket")
	RootCmd.PersistentFlags().StringVarP(&abci, "abci", "", "socket", "socket or grpc")
	RootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "print the command and results as if it were a console session")
}

func addQueryFlags() {
	queryCmd.PersistentFlags().StringVarP(&path, "path", "", "/store", "Path to prefix query with")
	queryCmd.PersistentFlags().IntVarP(&height, "height", "", 0, "Height to query the blockchain at")
	queryCmd.PersistentFlags().BoolVarP(&prove, "prove", "", false, "Whether or not to return a merkle proof of the query result")
}

func addCommands() {
	RootCmd.AddCommand(batchCmd)
	RootCmd.AddCommand(consoleCmd)
	RootCmd.AddCommand(echoCmd)
	RootCmd.AddCommand(infoCmd)
	RootCmd.AddCommand(setOptionCmd)
	RootCmd.AddCommand(deliverTxCmd)
	RootCmd.AddCommand(checkTxCmd)
	RootCmd.AddCommand(commitCmd)
	addQueryFlags()
	RootCmd.AddCommand(queryCmd)
}

var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Run a batch of abci commands against an application",
	Long:  "",
	Run:   cmdBatch,
}

var consoleCmd = &cobra.Command{
	Use:       "console",
	Short:     "Start an interactive abci console for multiple commands",
	Long:      "",
	ValidArgs: []string{"batch", "echo", "info", "set_option", "deliver_tx", "check_tx", "commit", "query"},
	Run:       cmdConsole,
}

var echoCmd = &cobra.Command{
	Use:   "echo",
	Short: "",
	Long:  "Have the application echo a message",
	Run:   cmdEcho,
}
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get some info about the application",
	Long:  "",
	Run:   cmdInfo,
}
var setOptionCmd = &cobra.Command{
	Use:   "set_option",
	Short: "Set an options on the application",
	Long:  "",
	Run:   cmdSetOption,
}

var deliverTxCmd = &cobra.Command{
	Use:   "deliver_tx",
	Short: "Deliver a new tx to the application",
	Long:  "",
	Run:   cmdDeliverTx,
}

var checkTxCmd = &cobra.Command{
	Use:   "check_tx",
	Short: "Validate a tx",
	Long:  "",
	Run:   cmdCheckTx,
}

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Commit the application state and return the Merkle root hash",
	Long:  "",
	Run:   cmdCommit,
}

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query the application state",
	Long:  "",
	Run:   cmdQuery,
}

// Generates new Args array based off of previous call args to maintain flag persistence
func persistentArgs(line []byte) []string {

	// generate the arguments to run from original os.Args
	// to maintain flag arguments
	args := os.Args
	args = args[:len(args)-1] // remove the previous command argument

	if len(line) > 0 { // prevents introduction of extra space leading to argument parse errors
		args = append(args, strings.Split(string(line), " ")...)
	}
	return args
}

//--------------------------------------------------------------------------------

func cmdBatch(cmd *cobra.Command, args []string) {
	bufReader := bufio.NewReader(os.Stdin)
	for {
		line, more, err := bufReader.ReadLine()
		if more {
			ifExit(errors.New("Input line is too long"))
		} else if err == io.EOF {
			break
		} else if len(line) == 0 {
			continue
		} else if err != nil {
			ifExit(err)
		}

		pArgs := persistentArgs(line)
		out, err := exec.Command(pArgs[0], pArgs[1:]...).Output()
		if err != nil {
			panic(err)
		}
		fmt.Println(string(out))
	}
}

func cmdConsole(cmd *cobra.Command, args []string) {

	for {
		fmt.Printf("\n> ")
		bufReader := bufio.NewReader(os.Stdin)
		line, more, err := bufReader.ReadLine()
		if more {
			ifExit(errors.New("Input is too long"))
		} else if err != nil {
			ifExit(err)
		}

		pArgs := persistentArgs(line)
		out, err := exec.Command(pArgs[0], pArgs[1:]...).Output()
		if err != nil {
			panic(err)
		}
		fmt.Println(string(out))
	}
}

// Have the application echo a message
func cmdEcho(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ifExit(errors.New("Command echo takes 1 argument"))
	}
	resEcho := client.EchoSync(args[0])
	printResponse(cmd, args, response{
		Data: resEcho.Data,
	})
}

// Get some info from the application
func cmdInfo(cmd *cobra.Command, args []string) {
	var version string
	if len(args) == 1 {
		version = args[0]
	}
	resInfo, err := client.InfoSync(types.RequestInfo{version})
	if err != nil {
		ifExit(err)
	}
	printResponse(cmd, args, response{
		Data: []byte(resInfo.Data),
	})
}

// Set an option on the application
func cmdSetOption(cmd *cobra.Command, args []string) {
	if len(args) != 2 {
		ifExit(errors.New("Command set_option takes 2 arguments (key, value)"))
	}
	resSetOption := client.SetOptionSync(args[0], args[1])
	printResponse(cmd, args, response{
		Log: resSetOption.Log,
	})
}

// Append a new tx to application
func cmdDeliverTx(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ifExit(errors.New("Command deliver_tx takes 1 argument"))
	}
	txBytes, err := stringOrHexToBytes(args[0])
	if err != nil {
		ifExit(err)
	}
	res := client.DeliverTxSync(txBytes)
	printResponse(cmd, args, response{
		Code: res.Code,
		Data: res.Data,
		Log:  res.Log,
	})
}

// Validate a tx
func cmdCheckTx(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ifExit(errors.New("Command check_tx takes 1 argument"))
	}
	txBytes, err := stringOrHexToBytes(args[0])
	if err != nil {
		ifExit(err)
	}
	res := client.CheckTxSync(txBytes)
	printResponse(cmd, args, response{
		Code: res.Code,
		Data: res.Data,
		Log:  res.Log,
	})
}

// Get application Merkle root hash
func cmdCommit(cmd *cobra.Command, args []string) {
	res := client.CommitSync()
	printResponse(cmd, args, response{
		Code: res.Code,
		Data: res.Data,
		Log:  res.Log,
	})
}

// Query application state
func cmdQuery(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		ifExit(errors.New("Command query takes 1 argument, the query bytes"))
	}

	queryBytes, err := stringOrHexToBytes(args[0])
	if err != nil {
		ifExit(err)
	}

	resQuery, err := client.QuerySync(types.RequestQuery{
		Data:   queryBytes,
		Path:   path,
		Height: uint64(height),
		Prove:  prove,
	})
	if err != nil {
		ifExit(err)
	}
	printResponse(cmd, args, response{
		Code: resQuery.Code,
		Log:  resQuery.Log,
		Query: &queryResponse{
			Key:    resQuery.Key,
			Value:  resQuery.Value,
			Height: resQuery.Height,
			Proof:  resQuery.Proof,
		},
	})
}

//--------------------------------------------------------------------------------

func printResponse(cmd *cobra.Command, args []string, rsp response) {

	if verbose {
		fmt.Println(">", cmd.Use, strings.Join(args, " "))
	}

	if !rsp.Code.IsOK() {
		fmt.Printf("-> code: %s\n", rsp.Code.String())
	}
	if len(rsp.Data) != 0 {
		fmt.Printf("-> data: %s\n", rsp.Data)
		fmt.Printf("-> data.hex: %X\n", rsp.Data)
	}
	if rsp.Log != "" {
		fmt.Printf("-> log: %s\n", rsp.Log)
	}

	if rsp.Query != nil {
		fmt.Printf("-> height: %d\n", rsp.Query.Height)
		if rsp.Query.Key != nil {
			fmt.Printf("-> key: %s\n", rsp.Query.Key)
			fmt.Printf("-> key.hex: %X\n", rsp.Query.Key)
		}
		if rsp.Query.Value != nil {
			fmt.Printf("-> value: %s\n", rsp.Query.Value)
			fmt.Printf("-> value.hex: %X\n", rsp.Query.Value)
		}
		if rsp.Query.Proof != nil {
			fmt.Printf("-> proof: %X\n", rsp.Query.Proof)
		}
	}
}

// NOTE: s is interpreted as a string unless prefixed with 0x
func stringOrHexToBytes(s string) ([]byte, error) {
	if len(s) > 2 && strings.ToLower(s[:2]) == "0x" {
		b, err := hex.DecodeString(s[2:])
		if err != nil {
			err = fmt.Errorf("Error decoding hex argument: %s", err.Error())
			return nil, err
		}
		return b, nil
	}

	if !strings.HasPrefix(s, "\"") || !strings.HasSuffix(s, "\"") {
		err := fmt.Errorf("Invalid string arg: \"%s\". Must be quoted or a \"0x\"-prefixed hex string", s)
		return nil, err
	}

	return []byte(s[1 : len(s)-1]), nil
}

func ifExit(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
