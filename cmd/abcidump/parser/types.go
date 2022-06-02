package parser

import (
	"fmt"
	"io"
	"reflect"

	"github.com/gogo/protobuf/proto"
)

// NewMessageType loads protobuf message type `typeName` and
// allocates new instance of this type.
func NewMessageType(typeName string) (proto.Message, error) {
	if msgType := proto.MessageType(typeName); msgType != nil {
		value := reflect.New(msgType.Elem())
		if msg, ok := value.Interface().(proto.Message); ok {
			return msg, nil
		}
		return nil, fmt.Errorf("invalid message type %T, expected child of proto.Message", msgType)
	}
	return nil, fmt.Errorf("message type %s not found", typeName)
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
