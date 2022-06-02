package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/cmd/abcidump/parser"
)

const (
	formatHex    = "hex"
	formatFile   = "file"
	formatBase64 = "base64"

	flagFormat = "format"
	flagType   = "type"
	flagRaw    = "raw"
	flagInput  = "input"
)

type ParseCmd struct {
	cmd   *cobra.Command
	Input io.Reader

	// flags
	InputData    string
	InputFormat  string
	ProtobufType string
	Raw          bool
}

// Command returns Cobra command that will parse input data
func (parseCmd *ParseCmd) Command() *cobra.Command {
	if parseCmd.cmd != nil {
		return parseCmd.cmd
	}

	inputFormats := []string{formatHex, formatBase64, formatFile}

	parseCmd.cmd = &cobra.Command{
		Use:   "parse",
		Short: "parse ABCI Protobuf message of the given type",

		PreRunE: parseCmd.PreRunE,
		RunE:    parseCmd.RunE,
		PostRun: func(cmd *cobra.Command, args []string) {
			if parseCmd.Input != nil {
				if closer, ok := parseCmd.Input.(io.Closer); ok {
					closer.Close()
				}
			}
		},
	}

	parseCmd.cmd.Flags().StringVar(&parseCmd.InputData, flagInput, "",
		"string representing input data (or path to input file)")
	parseCmd.cmd.Flags().StringVar(&parseCmd.InputFormat, flagFormat, "",
		"input data format, one of: "+strings.Join(inputFormats, ", ")+"; defaults to raw bytes read from stdin",
	)
	parseCmd.cmd.Flags().StringVarP(&parseCmd.ProtobufType, flagType, "t", "tendermint.abci.Request",
		"name of protobuf type of the message")
	parseCmd.cmd.Flags().BoolVar(&parseCmd.Raw, flagRaw, false, "read as raw protobuf data (without length prefix)")

	return parseCmd.cmd
}

// PreRunE parses command line arguments
func (parseCmd *ParseCmd) PreRunE(cmd *cobra.Command, args []string) (err error) {
	if parseCmd.Input, err = loadInputData(parseCmd.InputData, parseCmd.InputFormat); err != nil {
		return err
	}
	if parseCmd.Input != nil {
		parseCmd.cmd.SetIn(parseCmd.Input)
	}

	return nil
}

func loadInputData(input, format string) (reader io.Reader, err error) {
	switch format {
	case formatHex:
		var data []byte
		if data, err = hex.DecodeString(input); err != nil {
			return nil, fmt.Errorf("input is not valid hex data: %w", err)
		}
		return bytes.NewBuffer(data), nil

	case formatBase64:
		var data []byte
		if data, err = base64.StdEncoding.DecodeString(input); err != nil {
			return nil, fmt.Errorf("input is not valid base64 standard encoding: %w", err)
		}
		return bytes.NewBuffer(data), nil

	case formatFile:
		if reader, err = os.Open(input); err != nil {
			return nil, fmt.Errorf("cannot read file %s: %w", input, err)
		}
		return reader, nil

	case "", "-":
		return nil, nil

	default:
		return nil, fmt.Errorf("unsupported input format: %s", format)
	}
}

// RunE executes parsing logic
func (parseCmd *ParseCmd) RunE(cmd *cobra.Command, args []string) error {
	var err error

	parser := parser.NewParser(cmd.InOrStdin())
	parser.Out = cmd.OutOrStdout()
	parser.LengthDelimeted = !parseCmd.Raw

	for err != io.EOF {
		err = parser.Parse(parseCmd.ProtobufType)
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Error("cannot parse message", "error", err)
			continue
		}
	}

	return nil
}
