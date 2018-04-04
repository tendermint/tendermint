package common

import (
	fmt "fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorPanic(t *testing.T) {
	type pnk struct {
		msg string
	}

	capturePanic := func() (err Error) {
		defer func() {
			if r := recover(); r != nil {
				err = ErrorWrap(r, "This is the message in ErrorWrap(r, message).")
			}
			return
		}()
		panic(pnk{"something"})
		return nil
	}

	var err = capturePanic()

	assert.Equal(t, pnk{"something"}, err.Cause())
	assert.Equal(t, pnk{"something"}, err.T())
	assert.Equal(t, "This is the message in ErrorWrap(r, message).", err.Message())
	assert.Equal(t, "Error{`This is the message in ErrorWrap(r, message).` (cause: {something})}", fmt.Sprintf("%v", err))
	assert.Contains(t, fmt.Sprintf("%#v", err), "Message: This is the message in ErrorWrap(r, message).")
	assert.Contains(t, fmt.Sprintf("%#v", err), "Stack Trace:\n    0")
}

func TestErrorWrapSomething(t *testing.T) {

	var err = ErrorWrap("something", "formatter%v%v", 0, 1)

	assert.Equal(t, "something", err.Cause())
	assert.Equal(t, "something", err.T())
	assert.Equal(t, "formatter01", err.Message())
	assert.Equal(t, "Error{`formatter01` (cause: something)}", fmt.Sprintf("%v", err))
	assert.Regexp(t, `Message: formatter01\n`, fmt.Sprintf("%#v", err))
	assert.Contains(t, fmt.Sprintf("%#v", err), "Stack Trace:\n    0")
}

func TestErrorWrapNothing(t *testing.T) {

	var err = ErrorWrap(nil, "formatter%v%v", 0, 1)

	assert.Equal(t, nil, err.Cause())
	assert.Equal(t, nil, err.T())
	assert.Equal(t, "formatter01", err.Message())
	assert.Equal(t, "Error{`formatter01`}", fmt.Sprintf("%v", err))
	assert.Regexp(t, `Message: formatter01\n`, fmt.Sprintf("%#v", err))
	assert.Contains(t, fmt.Sprintf("%#v", err), "Stack Trace:\n    0")
}

func TestErrorNewError(t *testing.T) {

	var err = NewError("formatter%v%v", 0, 1)

	assert.Equal(t, nil, err.Cause())
	assert.Equal(t, "formatter%v%v", err.T())
	assert.Equal(t, "formatter01", err.Message())
	assert.Equal(t, "Error{`formatter01`}", fmt.Sprintf("%v", err))
	assert.Regexp(t, `Message: formatter01\n`, fmt.Sprintf("%#v", err))
	assert.NotContains(t, fmt.Sprintf("%#v", err), "Stack Trace")
}

func TestErrorNewErrorWithStacktrace(t *testing.T) {

	var err = NewError("formatter%v%v", 0, 1).Stacktrace()

	assert.Equal(t, nil, err.Cause())
	assert.Equal(t, "formatter%v%v", err.T())
	assert.Equal(t, "formatter01", err.Message())
	assert.Equal(t, "Error{`formatter01`}", fmt.Sprintf("%v", err))
	assert.Regexp(t, `Message: formatter01\n`, fmt.Sprintf("%#v", err))
	assert.Contains(t, fmt.Sprintf("%#v", err), "Stack Trace:\n    0")
}

func TestErrorNewErrorWithTrace(t *testing.T) {

	var err = NewError("formatter%v%v", 0, 1)
	err.Trace("trace %v", 1)
	err.Trace("trace %v", 2)
	err.Trace("trace %v", 3)

	assert.Equal(t, nil, err.Cause())
	assert.Equal(t, "formatter%v%v", err.T())
	assert.Equal(t, "formatter01", err.Message())
	assert.Equal(t, "Error{`formatter01`}", fmt.Sprintf("%v", err))
	assert.Regexp(t, `Message: formatter01\n`, fmt.Sprintf("%#v", err))
	dump := fmt.Sprintf("%#v", err)
	assert.NotContains(t, dump, "Stack Trace")
	assert.Regexp(t, `common/errors_test\.go:[0-9]+ - trace 1`, dump)
	assert.Regexp(t, `common/errors_test\.go:[0-9]+ - trace 2`, dump)
	assert.Regexp(t, `common/errors_test\.go:[0-9]+ - trace 3`, dump)
}

func TestErrorWrapError(t *testing.T) {
	var err1 error = NewError("my message")
	var err2 error = ErrorWrap(err1, "another message")
	assert.Equal(t, err1, err2)
}
