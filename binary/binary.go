package binary

import "io"

type Binary interface {
    WriteTo(io.Writer) (int64, error)
}
