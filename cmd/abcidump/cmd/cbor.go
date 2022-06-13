package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/spf13/cobra"
)

// CborCmd parses data encoded in CBOR format
type CborCmd struct {
	cmd   *cobra.Command
	Input io.Reader

	// flags
	InputData   string
	InputFormat string
}

// Command returns Cobra command
func (cborCmd *CborCmd) Command() *cobra.Command {
	if cborCmd.cmd != nil {
		return cborCmd.cmd
	}
	cborCmd.cmd = &cobra.Command{
		Use:   "cbor",
		Short: "Decoder for CBOR-encoded messages",
	}

	cborDecodeCmd := &cobra.Command{
		Use:     "decode",
		Short:   "Decode a CBOR-encoded message",
		PreRunE: cborCmd.PreRunE,
		RunE:    cborCmd.RunE,
		PostRun: func(cmd *cobra.Command, args []string) {
			if cborCmd.Input != nil {
				if closer, ok := cborCmd.Input.(io.Closer); ok {
					closer.Close()
				}
			}
		},
	}
	inputFormats := strings.Join([]string{formatHex, formatBase64, formatFile}, ",")
	cborDecodeCmd.Flags().StringVar(&cborCmd.InputFormat, flagFormat, "",
		"input data format, one of: "+inputFormats+"; defaults to raw bytes read from stdin",
	)
	cborDecodeCmd.Flags().StringVar(&cborCmd.InputData, flagInput, "", "filename or string representing input data")

	cborCmd.cmd.AddCommand(cborDecodeCmd)
	return cborCmd.cmd
}

// PreRunE parses command line arguments
func (cborCmd *CborCmd) PreRunE(_ *cobra.Command, args []string) (err error) {
	if cborCmd.Input, err = loadInputData(cborCmd.InputData, cborCmd.InputFormat); err != nil {
		return err
	}
	if cborCmd.Input != nil {
		cborCmd.cmd.SetIn(cborCmd.Input)
	}

	return nil
}

const jsonIndent = '\t'

// marshal recursively marshals provided data to JSON format
func marshal(data interface{}, indent []byte, out io.Writer) error {
	if len(indent) == 0 {
		indent = []byte{jsonIndent}
	}

	switch typed := data.(type) {
	// json.Marshal cannot handle map[interface{}]interface{}
	case map[interface{}]interface{}:
		return marshalMap(typed, indent, out)
	default:
		marshaled, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("cannot parse %+v: %w", data, err)
		}
		if _, err = out.Write(marshaled); err != nil {
			return err
		}
	}

	return nil
}

func marshalMap(mapToMarshal map[interface{}]interface{}, indent []byte, out io.Writer) error {
	// adding prefix; indent already added by the parent
	if _, err := out.Write([]byte{'{', '\n'}); err != nil {
		return err
	}

	i := 0
	for key, val := range mapToMarshal {
		// indentation
		if _, err := out.Write(indent); err != nil {
			return err
		}

		// Write "KEY":"VALUE"
		if err := marshal(key, append(indent, jsonIndent), out); err != nil {
			return fmt.Errorf("cannot marshal key %+v: %w", key, err)
		}
		if _, err := out.Write([]byte{':', ' '}); err != nil {
			return err
		}
		if err := marshal(val, append(indent, jsonIndent), out); err != nil {
			return fmt.Errorf("cannot marshal value %+v: %w", val, err)
		}

		// comma if needed, and new line
		if i < len(mapToMarshal)-1 { // not last
			if _, err := out.Write([]byte{','}); err != nil {
				return err
			}
		}
		if _, err := out.Write([]byte{'\n'}); err != nil {
			return err
		}
		i++
	}

	// suffix
	var suffix []byte
	if len(indent) > 1 {
		suffix = indent[:len(indent)-1]
		suffix = append(suffix, '}')
	} else { // top level
		suffix = []byte{'}', '\n'}
	}
	if _, err := out.Write(suffix); err != nil {
		return err
	}

	return nil
}

// RunE executes main logic of this command
func (cborCmd *CborCmd) RunE(_ *cobra.Command, args []string) error {
	data, err := io.ReadAll(cborCmd.Input)
	if err != nil {
		return fmt.Errorf("cannot read data: %w", err)
	}
	unmarshaled := map[interface{}]interface{}{}
	if err := cbor.Unmarshal(data, &unmarshaled); err != nil {
		return err
	}
	out := cborCmd.cmd.OutOrStdout()
	if err = marshal(unmarshaled, []byte{jsonIndent}, out); err != nil {
		return err
	}

	return nil
}
