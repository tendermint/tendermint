package parser

import (
	"bytes"
	"io"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/abci/types"
)

// Parser reads protobuf data from In, parses it and writes to Out in JSON format
type Parser struct {
	In              io.Reader // Input for the parser
	Out             io.Writer // Output for the parser
	LengthDelimeted bool      // If true, each input message should start with message length
	marshaler       jsonpb.Marshaler
}

func NewParser(in io.Reader) Parser {
	return Parser{
		marshaler:       jsonpb.Marshaler{Indent: "\t"},
		In:              in,
		Out:             &bytes.Buffer{},
		LengthDelimeted: true,
	}
}

// Parse parses next element as protobuf message type `msgType`.
// Returns io.EOF when there are no more messages in In.
func (p Parser) Parse(msgType string) error {

	msg, err := NewMessageType(msgType)
	if err != nil {
		return err
	}
	if err := readAndParse(p.In, p.LengthDelimeted, msg); err != nil {
		return err
	}
	if err := p.marshaler.Marshal(p.Out, msg); err != nil {
		return err
	}
	if _, err := p.Out.Write([]byte{'\n'}); err != nil {
		return nil
	}

	return nil
}

func readAndParse(in io.Reader, lengthDelimeted bool, dest proto.Message) error {
	if !lengthDelimeted {
		if err := readRaw(in, dest); err != nil {
			return err
		}
	} else {
		if err := types.ReadMessage(in, dest); err != nil {
			return err
		}
	}

	return nil
}
