package parser

import (
	"bytes"
	"io"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"

	"github.com/tendermint/tendermint/abci/types"
)

type Marshaler interface {
	Marshal(out io.Writer, pb proto.Message) error
}
type Parser struct {
	In              io.Reader
	Out             io.Writer
	LengthDelimeted bool
	Marshaler       Marshaler
}

func NewParser(in io.Reader) Parser {
	return Parser{
		Marshaler:       &jsonpb.Marshaler{Indent: "\t"},
		In:              in,
		Out:             &bytes.Buffer{},
		LengthDelimeted: true,
	}
}

func (p Parser) Parse(msgType string) error {

	msg, err := NewMessageType(msgType)
	if err != nil {
		return err
	}
	if err := readAndParse(p.In, p.LengthDelimeted, msg); err != nil {
		return err
	}
	if err := p.Marshaler.Marshal(p.Out, msg); err != nil {
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

	return nil // parseCmd.parse(msg)
}
