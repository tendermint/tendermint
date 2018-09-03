package log

import (
	"bytes"
	"fmt"
	"io"
	"sync"
	"time"

	kitlog "github.com/go-kit/kit/log"
	kitlevel "github.com/go-kit/kit/log/level"
	"github.com/go-logfmt/logfmt"
)

type tmfmtEncoder struct {
	*logfmt.Encoder
	buf bytes.Buffer
}

func (l *tmfmtEncoder) Reset() {
	l.Encoder.Reset()
	l.buf.Reset()
}

var tmfmtEncoderPool = sync.Pool{
	New: func() interface{} {
		var enc tmfmtEncoder
		enc.Encoder = logfmt.NewEncoder(&enc.buf)
		return &enc
	},
}

type tmfmtLogger struct {
	w io.Writer
}

// NewTMFmtLogger returns a logger that encodes keyvals to the Writer in
// Tendermint custom format. Note complex types (structs, maps, slices)
// formatted as "%+v".
//
// Each log event produces no more than one call to w.Write.
// The passed Writer must be safe for concurrent use by multiple goroutines if
// the returned Logger will be used concurrently.
func NewTMFmtLogger(w io.Writer) kitlog.Logger {
	return &tmfmtLogger{w}
}

func (l tmfmtLogger) Log(keyvals ...interface{}) error {
	enc := tmfmtEncoderPool.Get().(*tmfmtEncoder)
	enc.Reset()
	defer tmfmtEncoderPool.Put(enc)

	const unknown = "unknown"
	lvl := "none"
	msg := unknown
	module := unknown

	// indexes of keys to skip while encoding later
	excludeIndexes := make([]int, 0)

	for i := 0; i < len(keyvals)-1; i += 2 {
		// Extract level
		if keyvals[i] == kitlevel.Key() {
			excludeIndexes = append(excludeIndexes, i)
			switch keyvals[i+1].(type) {
			case string:
				lvl = keyvals[i+1].(string)
			case kitlevel.Value:
				lvl = keyvals[i+1].(kitlevel.Value).String()
			default:
				panic(fmt.Sprintf("level value of unknown type %T", keyvals[i+1]))
			}
			// and message
		} else if keyvals[i] == msgKey {
			excludeIndexes = append(excludeIndexes, i)
			msg = keyvals[i+1].(string)
			// and module (could be multiple keyvals; if such case last keyvalue wins)
		} else if keyvals[i] == moduleKey {
			excludeIndexes = append(excludeIndexes, i)
			module = keyvals[i+1].(string)
		}
	}

	// Form a custom Tendermint line
	//
	// Example:
	//     D[05-02|11:06:44.322] Stopping AddrBook (ignoring: already stopped)
	//
	// Description:
	//     D										- first character of the level, uppercase (ASCII only)
	//     [05-02|11:06:44.322] - our time format (see https://golang.org/src/time/format.go)
	//     Stopping ...					- message
	enc.buf.WriteString(fmt.Sprintf("%c[%s] %-44s ", lvl[0]-32, time.Now().Format("01-02|15:04:05.000"), msg))

	if module != unknown {
		enc.buf.WriteString("module=" + module + " ")
	}

KeyvalueLoop:
	for i := 0; i < len(keyvals)-1; i += 2 {
		for _, j := range excludeIndexes {
			if i == j {
				continue KeyvalueLoop
			}
		}

		err := enc.EncodeKeyval(keyvals[i], keyvals[i+1])
		if err == logfmt.ErrUnsupportedValueType {
			enc.EncodeKeyval(keyvals[i], fmt.Sprintf("%+v", keyvals[i+1]))
		} else if err != nil {
			return err
		}
	}

	// Add newline to the end of the buffer
	if err := enc.EndRecord(); err != nil {
		return err
	}

	// The Logger interface requires implementations to be safe for concurrent
	// use by multiple goroutines. For this implementation that means making
	// only one call to l.w.Write() for each call to Log.
	if _, err := l.w.Write(enc.buf.Bytes()); err != nil {
		return err
	}
	return nil
}
