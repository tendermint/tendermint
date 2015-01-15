package common

import (
	"io"
)

type PrefixedReader struct {
	Prefix []byte
	reader io.Reader
}

func NewPrefixedReader(prefix []byte, reader io.Reader) *PrefixedReader {
	return &PrefixedReader{prefix, reader}
}

func (pr *PrefixedReader) Read(p []byte) (n int, err error) {
	if len(pr.Prefix) > 0 {
		read := copy(p, pr.Prefix)
		pr.Prefix = pr.Prefix[read:]
		return read, nil
	} else {
		return pr.reader.Read(p)
	}
}
