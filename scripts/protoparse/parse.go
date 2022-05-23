package main

import (
	"bytes"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"

	"github.com/tendermint/tendermint/abci/types"
)

const (
	flagFile   = "file"
	flagHex    = "hex"
	flagBase64 = "base64"
	flagType   = "type"
	flagRaw    = "raw"
	// flagFieldTypes = "field-types"
)

type ParseCmd struct {
	cmd          *cobra.Command
	Input        io.ReadCloser
	InputFile    string
	InputHex     string
	InputBase64  string
	InputData    []byte
	ProtobufType string

	FieldTypes []string
	FieldsMap  map[string]string

	Raw bool
}

// func (parseCmd *ParseCmd) fieldType(fieldName string) string {
// 	return parseCmd.FieldsMap[strings.ToLower(fieldName)]
// }

func (parseCmd *ParseCmd) Command() *cobra.Command {
	if parseCmd.cmd != nil {
		return parseCmd.cmd
	}

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
				parseCmd.Input.Close()
			}
		},
	}

	parseCmd.cmd.Flags().StringVar(&parseCmd.InputFile, flagFile, "", "input file")
	parseCmd.cmd.Flags().StringVar(&parseCmd.InputHex, flagHex, "", "input data in hex format")
	parseCmd.cmd.Flags().StringVar(&parseCmd.InputBase64, flagBase64, "", "input data in base64 format")
	parseCmd.cmd.Flags().StringVarP(&parseCmd.ProtobufType, flagType, "t", "tendermint.abci.Request",
		"protobuf type of the root message")
	parseCmd.cmd.Flags().BoolVar(&parseCmd.Raw, flagRaw, false, "read as raw protobuf data (without length prefix)")

	// parseCmd.cmd.Flags().StringSliceVarP(
	// 	&parseCmd.FieldTypes,
	// 	flagFieldTypes, "f",
	// 	[]string{},
	// 	"comma-separated list of fields to parse and their respective types (separated by : ), "+
	// 		"for example: info:tendermint.abci.Info,data:tendermint.abci.Data")

	return parseCmd.cmd
}

// PreRunE parses command line arguments
func (parseCmd *ParseCmd) PreRunE(cmd *cobra.Command, args []string) (err error) {

	// Input data
	if parseCmd.InputFile != "" {
		if parseCmd.Input, err = os.Open(parseCmd.InputFile); err != nil {
			return fmt.Errorf("cannot read file %s: %w", parseCmd.InputFile, err)
		}
	}

	if parseCmd.InputHex != "" {
		var data []byte
		if data, err = hex.DecodeString(parseCmd.InputHex); err != nil {
			return fmt.Errorf("cannot parse hex data provided with --%s: %w", flagHex, err)
		}
		parseCmd.Input = ioutil.NopCloser(bytes.NewBuffer(data))
	}

	if parseCmd.InputBase64 != "" {
		var data []byte
		if data, err = base64.StdEncoding.DecodeString(parseCmd.InputBase64); err != nil {
			return fmt.Errorf("cannot parse base64 data provided with --%s: %w", flagBase64, err)
		}
		parseCmd.Input = ioutil.NopCloser(bytes.NewBuffer(data))
	}

	if parseCmd.Input != nil {
		parseCmd.cmd.SetIn(parseCmd.Input)
	}

	// type mapping
	// if parseCmd.FieldsMap == nil {
	// 	parseCmd.FieldsMap = map[string]string{}
	// }

	for _, field := range parseCmd.FieldTypes {
		splitted := strings.Split(field, ":")
		if _, err := newProtoMessage(splitted[1]); err != nil {
			return fmt.Errorf("invalid field types mapping '%s': %w", field, err)
		}
		parseCmd.FieldsMap[strings.ToLower(splitted[0])] = splitted[1]
	}

	return nil
}

func (parseCmd *ParseCmd) RunE(cmd *cobra.Command, args []string) error {
	var (
		msg proto.Message
		// ok  bool
		err error
	)
	if msg, err = newProtoMessage(parseCmd.ProtobufType); err != nil {
		return fmt.Errorf("cannot load main message type: %w", err)
	}

	for err != io.EOF {
		err = parseCmd.readAndParse(msg)
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Error("cannot parse message", "error", err)
			continue
		}

		text, err := toJSON(msg)
		if err != nil {
			logger.Error("cannot format the message as JSON", "error", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "\"%T\": %s,\n", msg, text)
	}

	return nil
}

func readRaw(r io.Reader, msg proto.Message) error {
	var (
		data []byte
		err  error
	)

	if data, err = io.ReadAll(r); err != nil {
		return fmt.Errorf("cannot read message: %w", err)
	}
	if len(data) == 0 {
		return io.EOF
	}
	err = proto.Unmarshal(data, msg)
	return err
}

func (parseCmd *ParseCmd) readAndParse(msg proto.Message) error {
	if parseCmd.Raw {
		if err := readRaw(parseCmd.cmd.InOrStdin(), msg); err != nil {
			return err
		}
	} else {
		if err := types.ReadMessage(parseCmd.cmd.InOrStdin(), msg); err != nil {
			return err
		}
	}

	return nil // parseCmd.parse(msg)
}

/* This is not working
func (parseCmd *ParseCmd) parse(msg interface{}) error {
	// Check fields and parse them if needed
	msgValue := reflect.Indirect(reflect.ValueOf(msg))
	msgType := reflect.Indirect(msgValue).Type()

	for i := 0; i < msgType.NumField(); i++ {
		field := msgValue.Field(i)
		fieldName := msgType.Field(i).Name
		// fieldType := msgType.Field(i)
		// fieldValue := reflect.Indirect(field)
		iface := field.Interface()

		fieldValue := reflect.Indirect(reflect.ValueOf(iface))
		fieldType := fieldValue.Type()
		_ = iface

		logger.Debug("checking field", "field", fieldType, "name", fieldName, "kind", fieldType.Kind().String())

		if fieldType.Kind() == reflect.Struct {
			if err := parseCmd.parse(fieldValue.Interface()); err != nil {
				return fmt.Errorf("cannot interpret struct field %s: %w", fieldType.Name(), err)
			}
			continue
		}
		protoFieldType := parseCmd.fieldType(fieldName)
		if protoFieldType == "" {
			continue
		}

		// we assume this is a text field
		if field.Kind() != reflect.Slice {
			return fmt.Errorf("field %s is %s, expected string", fieldName, fieldType.Kind().String())
		}
		// raw := msgValue.Bytes()
		raw := field.Bytes()
		fmt.Println(raw)
		// raw, err := base64.StdEncoding.DecodeString(b64)
		// if err != nil {
		// 	return fmt.Errorf("cannot base64-decode field %s value %s: %w", fieldName, b64, err)
		// }
		// raw := []byte{}

		protoField, err := newMsg(protoFieldType)
		if err != nil {
			return fmt.Errorf("protobuf type %s for field %s not found: %w", protoFieldType, fieldName, err)
		}
		err = proto.Unmarshal(raw, protoField)
		if err != nil {
			return fmt.Errorf("cannot unmarshal field %s to type %s: %w", fieldName, protoFieldType, err)
		}

		logger.Debug("unmarshaled field", "field", fieldType.Name, "protobuf_type", protoFieldType, "value", protoField)
	}

	return nil
}
*/

// newProtoMessage creates new proto.Message object of type `typ`
func newProtoMessage(typ string) (msg proto.Message, err error) {
	if msgType := proto.MessageType(typ); msgType != nil {
		value := reflect.New(msgType.Elem())
		var ok bool
		if msg, ok = value.Interface().(proto.Message); !ok {
			return nil, fmt.Errorf("invalid message type %T, expected child of proto.Message", msgType)
		}
	} else {
		return nil, fmt.Errorf("message type %s not found", typ)
	}

	logger.Debug("loaded protobuf message type", "msg", msg, "type", fmt.Sprintf("%T", msg))
	return msg, nil
}

func toJSON(msg proto.Message) (string, error) {
	marshaler := jsonpb.Marshaler{
		Indent: "\t",
	}
	return marshaler.MarshalToString(msg)
}
