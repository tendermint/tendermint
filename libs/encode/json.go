package encode

import (
	"bytes"
	"errors"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
)

// MarshalJSON provides an auxiliary function to return Proto3 JSON encoded
// bytes of a message.
func MarshalJSON(msg proto.Message) ([]byte, error) {
	jm := &jsonpb.Marshaler{EmitDefaults: false, OrigName: false}
	buf := new(bytes.Buffer)

	if err := jm.Marshal(buf, msg); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// UnmarshalJSON provides an auxiliary function to return bytes from Proto3 JSON
// message
func UnmarshalJSON(data []byte) (pb proto.Message, err error) {
	if data == nil {
		return nil, errors.New("nil data")
	}
	jm := &jsonpb.Unmarshaler{}

	reader := bytes.NewBuffer(data)

	err = jm.Unmarshal(reader, pb)

	return
}
