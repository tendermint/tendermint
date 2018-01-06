package tmencoding

import (
	"github.com/tendermint/go-wire/nowriter/tmlegacy"
	"io"
	"time"
)

// Legacy interface following old code as closely as possible.
// Changed `WriteBytes` to `WriteOctets` to solve `go vet` issue.
// The explicit declaration of this interface provides a verbal
// discussion tool as well as a code marking work and it allows
// us to migrate away from using the global namespace for the Write*
// methods in the old 'wire.go' as we refactor.  This class may
// disappear once the refactoring is complete.
type TMEncoderFastIOWriter interface {
	WriteBool(b bool, w io.Writer, n *int, err *error)
	WriteFloat32(f float32, w io.Writer, n *int, err *error)
	WriteFloat64(f float64, w io.Writer, n *int, err *error)
	WriteInt8(i int8, w io.Writer, n *int, err *error)
	WriteInt16(i int16, w io.Writer, n *int, err *error)
	WriteInt32(i int32, w io.Writer, n *int, err *error)
	WriteInt64(i int64, w io.Writer, n *int, err *error)
	WriteOctet(b byte, w io.Writer, n *int, err *error)
	WriteOctetSlice(bz []byte, w io.Writer, n *int, err *error)
	WriteTime(t time.Time, w io.Writer, n *int, err *error)
	WriteTo(bz []byte, w io.Writer, n *int, err *error)
	WriteUint8(i uint8, w io.Writer, n *int, err *error)
	WriteUint16(i uint16, w io.Writer, n *int, err *error)
	WriteUint16s(iz []uint16, w io.Writer, n *int, err *error)
	WriteUint32(i uint32, w io.Writer, n *int, err *error)
	WriteUint64(i uint64, w io.Writer, n *int, err *error)
	WriteUvarint(i uint, w io.Writer, n *int, err *error)
	WriteVarint(i int, w io.Writer, n *int, err *error)
}

var _ TMEncoderFastIOWriter = (*tmlegacy.TMEncoderLegacy)(nil) // complete
