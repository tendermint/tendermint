package main

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	cmn "github.com/tendermint/tmlibs/common"
	"github.com/tendermint/tmlibs/log"

	abcicli "github.com/tendermint/abci/client"
	"github.com/tendermint/abci/example/code"
	"github.com/tendermint/abci/example/counter"
	"github.com/tendermint/abci/example/dummy"
	"github.com/tendermint/abci/server"
	servertest "github.com/tendermint/abci/tests/server"
	"github.com/tendermint/abci/types"
	"github.com/tendermint/abci/version"
)

// client is a global variable so it can be reused by the console
var (
	client abcicli.Client
	logger log.Logger
)

// flags
var (
	// global
	flagAddress  string
	flagAbci     string
	flagVerbose  bool   // for the println output
	flagLogLevel string // for the logger

	// query
	flagPath   string
	flagHeight int
	flagProve  bool

	// counter
	flagAddrC  string
	flagSerial bool

	// dummy
	flagAddrD   string
	flagPersist string
)

var RootCmd = &cobra.Command{
	Use:   "abci-cli",
	Short: "",
	Long:  "",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {

		switch cmd.Use {
		case "counter", "dummy": // for the examples apps, don't pre-run
			return nil
		case "version": // skip running for version command
			return nil
		}

		if logger == nil {
			allowLevel, err := log.AllowLevel(flagLogLevel)
			if err != nil {
				return err
			}
			logger = log.NewFilter(log.NewTMLogger(log.NewSyncWriter(os.Stdout)), allowLevel)
		}
		if client == nil {
			var err error
			client, err = abcicli.NewClient(flagAddress, flagAbci, false)
			if err != nil {
				return err
			}
			client.SetLogger(logger.With("module", "abci-client"))
			if err := client.Start(); err != nil {
				return err
			}
		}
		return nil
	},
}

// Structure for data passed to print response.
type response struct {
	// generic abci response
	Data []byte
	Code uint32
	Log  string

	Query *queryResponse
}

type queryResponse struct {
	Key    []byte
	Value  []byte
	Height int64
	Proof  []byte
}

func Execute() error {
	addGlobalFlags()
	addCommands()
	return RootCmd.Execute()
}

func addGlobalFlags() {
	RootCmd.PersistentFlags().StringVarP(&flagAddress, "address", "", "tcp://0.0.0.0:46658", "Address of application socket")
	RootCmd.PersistentFlags().StringVarP(&flagAbci, "abci", "", "socket", "Either socket or grpc")
	RootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "Print the command and results as if it were a console session")
	RootCmd.PersistentFlags().StringVarP(&flagLogLevel, "log_level", "", "debug", "Set the logger level")
}

func addQueryFlags() {
	queryCmd.PersistentFlags().StringVarP(&flagPath, "path", "", "/store", "Path to prefix query with")
	queryCmd.PersistentFlags().IntVarP(&flagHeight, "height", "", 0, "Height to query the blockchain at")
	queryCmd.PersistentFlags().BoolVarP(&flagProve, "prove", "", false, "Whether or not to return a merkle proof of the query result")
}

func addCounterFlags() {
	counterCmd.PersistentFlags().StringVarP(&flagAddrC, "addr", "", "tcp://0.0.0.0:46658", "Listen address")
	counterCmd.PersistentFlags().BoolVarP(&flagSerial, "serial", "", false, "Enforce incrementing (serial) transactions")
}

func addDummyFlags() {
	dummyCmd.PersistentFlags().StringVarP(&flagAddrD, "addr", "", "tcp://0.0.0.0:46658", "Listen address")
	dummyCmd.PersistentFlags().StringVarP(&flagPersist, "persist", "", "", "Directory to use for a database")
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
	RootCmd.AddCommand(versionCmd)
	RootCmd.AddCommand(testCmd)
	addQueryFlags()
	RootCmd.AddCommand(queryCmd)

	// examples
	addCounterFlags()
	RootCmd.AddCommand(counterCmd)
	addDummyFlags()
	RootCmd.AddCommand(dummyCmd)
}

var batchCmd = &cobra.Command{
	Use:   "batch",
	Short: "Run a batch of abci commands against an application",
	Long:  "",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmdBatch(cmd, args)
	},
}

var consoleCmd = &cobra.Command{
	Use:       "console",
	Short:     "Start an interactive abci console for multiple commands",
	Long:      "",
	Args:      cobra.ExactArgs(0),
	ValidArgs: []string{"batch", "echo", "info", "set_option", "deliver_tx", "check_tx", "commit", "query"},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmdConsole(cmd, args)
	},
}

var echoCmd = &cobra.Command{
	Use:   "echo",
	Short: "Have the application echo a message",
	Long:  "",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmdEcho(cmd, args)
	},
}
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get some info about the application",
	Long:  "",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmdInfo(cmd, args)
	},
}
var setOptionCmd = &cobra.Command{
	Use:   "set_option",
	Short: "Set an option on the application",
	Long:  "",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmdSetOption(cmd, args)
	},
}

var deliverTxCmd = &cobra.Command{
	Use:   "deliver_tx",
	Short: "Deliver a new transaction to the application",
	Long:  "",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmdDeliverTx(cmd, args)
	},
}

var checkTxCmd = &cobra.Command{
	Use:   "check_tx",
	Short: "Validate a transaction",
	Long:  "",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmdCheckTx(cmd, args)
	},
}

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Commit the application state and return the Merkle root hash",
	Long:  "",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmdCommit(cmd, args)
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print abci console version",
	Long:  "",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(version.Version)
		return nil
	},
}

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Query the application state",
	Long:  "",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmdQuery(cmd, args)
	},
}

var counterCmd = &cobra.Command{
	Use:   "counter",
	Short: "ABCI demo example",
	Long:  "",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmdCounter(cmd, args)
	},
}

var dummyCmd = &cobra.Command{
	Use:   "dummy",
	Short: "ABCI demo example",
	Long:  "",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmdDummy(cmd, args)
	},
}

var testCmd = &cobra.Command{
	Use:   "test",
	Short: "Run integration tests",
	Long:  "",
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmdTest(cmd, args)
	},
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

func or(err1 error, err2 error) error {
	if err1 == nil {
		return err2
	} else {
		return err1
	}
}

func cmdTest(cmd *cobra.Command, args []string) error {
	fmt.Println("Running tests")

	err := servertest.InitChain(client)
	fmt.Println("")
	err = or(err, servertest.SetOption(client, "serial", "on"))
	fmt.Println("")
	err = or(err, servertest.Commit(client, nil))
	fmt.Println("")
	err = or(err, servertest.DeliverTx(client, []byte("abc"), code.CodeTypeBadNonce, nil))
	fmt.Println("")
	err = or(err, servertest.Commit(client, nil))
	fmt.Println("")
	err = or(err, servertest.DeliverTx(client, []byte{0x00}, code.CodeTypeOK, nil))
	fmt.Println("")
	err = or(err, servertest.Commit(client, []byte{0, 0, 0, 0, 0, 0, 0, 1}))
	fmt.Println("")
	err = or(err, servertest.DeliverTx(client, []byte{0x00}, code.CodeTypeBadNonce, nil))
	fmt.Println("")
	err = or(err, servertest.DeliverTx(client, []byte{0x01}, code.CodeTypeOK, nil))
	fmt.Println("")
	err = or(err, servertest.DeliverTx(client, []byte{0x00, 0x02}, code.CodeTypeOK, nil))
	fmt.Println("")
	err = or(err, servertest.DeliverTx(client, []byte{0x00, 0x03}, code.CodeTypeOK, nil))
	fmt.Println("")
	err = or(err, servertest.DeliverTx(client, []byte{0x00, 0x00, 0x04}, code.CodeTypeOK, nil))
	fmt.Println("")
	err = or(err, servertest.DeliverTx(client, []byte{0x00, 0x00, 0x06}, code.CodeTypeBadNonce, nil))
	fmt.Println("")
	err = or(err, servertest.Commit(client, []byte{0, 0, 0, 0, 0, 0, 0, 5}))

	if err != nil {
		return errors.New("Some checks didn't pass, please inspect stdout to see the exact failures.")
	}
	return nil
}

func cmdBatch(cmd *cobra.Command, args []string) error {
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

		cmdArgs := persistentArgs(line)
		if err := muxOnCommands(cmd, cmdArgs); err != nil {
			return err
		}
		fmt.Println()
	}
	return nil
}

func cmdConsole(cmd *cobra.Command, args []string) error {
	for {
		fmt.Printf("> ")
		bufReader := bufio.NewReader(os.Stdin)
		line, more, err := bufReader.ReadLine()
		if more {
			return errors.New("Input is too long")
		} else if err != nil {
			return err
		}

		pArgs := persistentArgs(line)
		if err := muxOnCommands(cmd, pArgs); err != nil {
			return err
		}
	}
	return nil
}

var (
	errBadPersistentArgs = errors.New("expecting persistent args of the form: abci-cli [command] <...>")

	errUnimplemented = errors.New("unimplemented")
)

func muxOnCommands(cmd *cobra.Command, pArgs []string) error {
	if len(pArgs) < 2 {
		return errBadPersistentArgs
	}
	subCommand, actualArgs := pArgs[1], pArgs[2:]

	switch strings.ToLower(subCommand) {
	case "batch":
		return muxOnCommands(cmd, actualArgs)
	case "check_tx":
		return cmdCheckTx(cmd, actualArgs)
	case "commit":
		return cmdCommit(cmd, actualArgs)
	case "deliver_tx":
		return cmdDeliverTx(cmd, actualArgs)
	case "echo":
		return cmdEcho(cmd, actualArgs)
	case "info":
		return cmdInfo(cmd, actualArgs)
	case "query":
		return cmdQuery(cmd, actualArgs)
	case "set_option":
		return cmdSetOption(cmd, actualArgs)
	default:
		return cmdUnimplemented(cmd, pArgs)
	}
}

func cmdUnimplemented(cmd *cobra.Command, args []string) error {
	// TODO: Print out all the sub-commands available
	msg := "unimplemented command"
	if err := cmd.Help(); err != nil {
		msg = err.Error()
	}
	if len(args) > 0 {
		msg += fmt.Sprintf(" args: [%s]", strings.Join(args, " "))
	}
	printResponse(cmd, args, response{
		Code: codeBad,
		Log:  msg,
	})
	return nil
}

// Have the application echo a message
func cmdEcho(cmd *cobra.Command, args []string) error {
	msg := ""
	if len(args) > 0 {
		msg = args[0]
	}
	res, err := client.EchoSync(msg)
	if err != nil {
		return err
	}
	printResponse(cmd, args, response{
		Data: []byte(res.Message),
	})
	return nil
}

// Get some info from the application
func cmdInfo(cmd *cobra.Command, args []string) error {
	var version string
	if len(args) == 1 {
		version = args[0]
	}
	res, err := client.InfoSync(types.RequestInfo{version})
	if err != nil {
		return err
	}
	printResponse(cmd, args, response{
		Data: []byte(res.Data),
	})
	return nil
}

const codeBad uint32 = 10

// Set an option on the application
func cmdSetOption(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		printResponse(cmd, args, response{
			Code: codeBad,
			Log:  "want at least arguments of the form: <key> <value>",
		})
		return nil
	}

	key, val := args[0], args[1]
	res, err := client.SetOptionSync(types.RequestSetOption{key, val})
	if err != nil {
		return err
	}
	printResponse(cmd, args, response{
		Code: res.Code,
		Log:  res.Log,
	})
	return nil
}

// Append a new tx to application
func cmdDeliverTx(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		printResponse(cmd, args, response{
			Code: codeBad,
			Log:  "want the tx",
		})
		return nil
	}
	txBytes, err := stringOrHexToBytes(args[0])
	if err != nil {
		return err
	}
	res, err := client.DeliverTxSync(txBytes)
	if err != nil {
		return err
	}
	printResponse(cmd, args, response{
		Code: res.Code,
		Data: res.Data,
		Log:  res.Log,
	})
	return nil
}

// Validate a tx
func cmdCheckTx(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		printResponse(cmd, args, response{
			Code: codeBad,
			Log:  "want the tx",
		})
		return nil
	}
	txBytes, err := stringOrHexToBytes(args[0])
	if err != nil {
		return err
	}
	res, err := client.CheckTxSync(txBytes)
	if err != nil {
		return err
	}
	printResponse(cmd, args, response{
		Code: res.Code,
		Data: res.Data,
		Log:  res.Log,
	})
	return nil
}

// Get application Merkle root hash
func cmdCommit(cmd *cobra.Command, args []string) error {
	res, err := client.CommitSync()
	if err != nil {
		return err
	}
	printResponse(cmd, args, response{
		Code: res.Code,
		Data: res.Data,
		Log:  res.Log,
	})
	return nil
}

// Query application state
func cmdQuery(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		printResponse(cmd, args, response{
			Code: codeBad,
			Log:  "want the query",
		})
		return nil
	}
	queryBytes, err := stringOrHexToBytes(args[0])
	if err != nil {
		return err
	}

	resQuery, err := client.QuerySync(types.RequestQuery{
		Data:   queryBytes,
		Path:   flagPath,
		Height: int64(flagHeight),
		Prove:  flagProve,
	})
	if err != nil {
		return err
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
	return nil
}

func cmdCounter(cmd *cobra.Command, args []string) error {

	app := counter.NewCounterApplication(flagSerial)

	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))

	// Start the listener
	srv, err := server.NewServer(flagAddrC, flagAbci, app)
	if err != nil {
		return err
	}
	srv.SetLogger(logger.With("module", "abci-server"))
	if err := srv.Start(); err != nil {
		return err
	}

	// Wait forever
	cmn.TrapSignal(func() {
		// Cleanup
		srv.Stop()
	})
	return nil
}

func cmdDummy(cmd *cobra.Command, args []string) error {
	logger := log.NewTMLogger(log.NewSyncWriter(os.Stdout))

	// Create the application - in memory or persisted to disk
	var app types.Application
	if flagPersist == "" {
		app = dummy.NewDummyApplication()
	} else {
		app = dummy.NewPersistentDummyApplication(flagPersist)
		app.(*dummy.PersistentDummyApplication).SetLogger(logger.With("module", "dummy"))
	}

	// Start the listener
	srv, err := server.NewServer(flagAddrD, flagAbci, app)
	if err != nil {
		return err
	}
	srv.SetLogger(logger.With("module", "abci-server"))
	if err := srv.Start(); err != nil {
		return err
	}

	// Wait forever
	cmn.TrapSignal(func() {
		// Cleanup
		srv.Stop()
	})
	return nil
}

//--------------------------------------------------------------------------------

func printResponse(cmd *cobra.Command, args []string, rsp response) {

	if flagVerbose {
		fmt.Println(">", cmd.Use, strings.Join(args, " "))
	}

	// Always print the status code.
	if rsp.Code == types.CodeTypeOK {
		fmt.Printf("-> code: OK\n")
	} else {
		fmt.Printf("-> code: %d\n", rsp.Code)

	}

	if len(rsp.Data) != 0 {
		// Do no print this line when using the commit command
		// because the string comes out as gibberish
		if cmd.Use != "commit" {
			fmt.Printf("-> data: %s\n", rsp.Data)
		}
		fmt.Printf("-> data.hex: 0x%X\n", rsp.Data)
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
