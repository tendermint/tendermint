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

	"github.com/tendermint/tendermint/scripts/abcidump/parser"
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

// func (parseCmd *ParseCmd) fieldType(fieldName string) string {
// 	return parseCmd.FieldsMap[strings.ToLower(fieldName)]
// }

func (parseCmd *ParseCmd) Command() *cobra.Command {
	if parseCmd.cmd != nil {
		return parseCmd.cmd
	}

	inputFormats := []string{formatHex, formatBase64, formatFile}

	parseCmd.cmd = &cobra.Command{
		Use:   "parse",
		Short: "parse request or response dumped to file with tcpdump",
		Long: "Parse request or response dumped to a file with tcpdump.\n" +
			"Example dump command:\n" +
			"\tdocker exec -ti mn_evo_services_drive_abci_1 " +
			"tcpdump -s 65535 -w /var/log/drive/dump-26658-`date -u -Is`.pcap port 26658\n",

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

	parseCmd.cmd.Flags().StringVar(&parseCmd.InputData, flagInput, "", "filename or string representing input data")
	parseCmd.cmd.Flags().StringVar(&parseCmd.InputFormat, flagFormat, "",
		"input data format, one of: "+strings.Join(inputFormats, ", ")+"; defaults to raw bytes read from stdin",
	)

	parseCmd.cmd.Flags().StringVarP(&parseCmd.ProtobufType, flagType, "t", "tendermint.abci.Request",
		"protobuf type of the root message")
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

	case "":
		return nil, nil

	default:
		return nil, fmt.Errorf("unsupported input format %s", format)
	}
}

func (parseCmd *ParseCmd) RunE(cmd *cobra.Command, args []string) error {
	var (
		// msg proto.Message
		// ok  bool
		err error
	)

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
