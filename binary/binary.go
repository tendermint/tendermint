package binary

import "io"

type Binary interface {
    ByteSize()      int
    WriteTo(io.Writer) (int64, error)
    Equals(Binary)  bool
}
