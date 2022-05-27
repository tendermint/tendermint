package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/spf13/cobra"
)

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
func (cborCmd *CborCmd) PreRunE(cmd *cobra.Command, args []string) (err error) {
	if cborCmd.Input, err = loadInputData(cborCmd.InputData, cborCmd.InputFormat); err != nil {
		return err
	}
	if cborCmd.Input != nil {
		cborCmd.cmd.SetIn(cborCmd.Input)
	}

	return nil
}

const jsonIndent = '\t'

func marshal(data interface{}, depth int) ([]byte, error) {
	switch typed := data.(type) {
	case map[interface{}]interface{}:
		ret := []byte("{")
		isFirst := true
		for key, val := range typed {

			if !isFirst {
				ret = append(ret, ',')
			} else {
				isFirst = false
			}
			ret = append(ret, '\n')

			k1, err := marshal(key, depth+1)
			if err != nil {
				return nil, err
			}
			v1, err := marshal(val, depth+1)
			if err != nil {
				return nil, err
			}
			for i := 0; i < depth; i++ {
				ret = append(ret, jsonIndent)
			}

			ret = append(ret, k1...)
			ret = append(ret, ':', ' ')
			ret = append(ret, v1...)
		}
		ret = append(ret, '\n', '}')
		return ret, nil

	default:
		s, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("cannot parse %s: %w", typed, err)
		}
		return s, nil
	}
}

// RunE executes main logic of this command
func (cborCmd *CborCmd) RunE(cmd *cobra.Command, args []string) error {
	data, err := io.ReadAll(cborCmd.Input)
	if err != nil {
		return fmt.Errorf("cannot read data: %w", err)
	}
	s := map[interface{}]interface{}{}
	if err := cbor.Unmarshal(data, &s); err != nil {
		return err
	}

	out := cborCmd.cmd.OutOrStdout()
	str, err := marshal(s, 1)
	if err != nil {
		return err
	}
	if _, err := out.Write(str); err != nil {
		return err
	}

	return nil
}
