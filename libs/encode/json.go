package encode

import (
	"bytes"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/gogo/protobuf/proto"
)

var (
	jmIndent = jsonpb.Marshaler{EmitDefaults: false, OrigName: false, Indent: "  "}
	jm       = jsonpb.Marshaler{EmitDefaults: false, OrigName: false}
)

// MarshalJSON provides an auxiliary function to return Proto3 JSON encoded
// bytes of a message.
func MarshalJSON(msg proto.Message) ([]byte, error) {
	buf := new(bytes.Buffer)

	if err := jm.Marshal(buf, msg); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// MarshalJSONIndent provides an auxiliary function to return Proto3 indented
// JSON encoded bytes of a message.
func MarshalJSONIndent(msg proto.Message) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := jmIndent.Marshal(buf, msg); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
