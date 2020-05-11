package encode

import (
	"bytes"

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

// MarshalJSONIndent provides an auxiliary function to return Proto3 indented
// JSON encoded bytes of a message.
func MarshalJSONIndent(msg proto.Message) ([]byte, error) {
	jm := &jsonpb.Marshaler{EmitDefaults: false, OrigName: false, Indent: "  "}
	buf := new(bytes.Buffer)

	if err := jm.Marshal(buf, msg); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
