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
		}()
		panic(pnk{"something"})
	}

	var err = capturePanic()

	assert.Equal(t, pnk{"something"}, err.Data())
	assert.Equal(t, "{something}", fmt.Sprintf("%v", err))
	assert.Contains(t, fmt.Sprintf("%#v", err), "This is the message in ErrorWrap(r, message).")
	assert.Contains(t, fmt.Sprintf("%#v", err), "Stack Trace:\n    0")
}

func TestErrorWrapSomething(t *testing.T) {

	var err = ErrorWrap("something", "formatter%v%v", 0, 1)

	assert.Equal(t, "something", err.Data())
	assert.Equal(t, "something", fmt.Sprintf("%v", err))
	assert.Regexp(t, `formatter01\n`, fmt.Sprintf("%#v", err))
	assert.Contains(t, fmt.Sprintf("%#v", err), "Stack Trace:\n    0")
}

func TestErrorWrapNothing(t *testing.T) {

	var err = ErrorWrap(nil, "formatter%v%v", 0, 1)

	assert.Equal(t,
		FmtError{"formatter%v%v", []interface{}{0, 1}},
		err.Data())
	assert.Equal(t, "formatter01", fmt.Sprintf("%v", err))
	assert.Contains(t, fmt.Sprintf("%#v", err), `Data: common.FmtError{format:"formatter%v%v", args:[]interface {}{0, 1}}`)
	assert.Contains(t, fmt.Sprintf("%#v", err), "Stack Trace:\n    0")
}

func TestErrorNewError(t *testing.T) {

	var err = NewError("formatter%v%v", 0, 1)

	assert.Equal(t,
		FmtError{"formatter%v%v", []interface{}{0, 1}},
		err.Data())
	assert.Equal(t, "formatter01", fmt.Sprintf("%v", err))
	assert.Contains(t, fmt.Sprintf("%#v", err), `Data: common.FmtError{format:"formatter%v%v", args:[]interface {}{0, 1}}`)
	assert.NotContains(t, fmt.Sprintf("%#v", err), "Stack Trace")
}

func TestErrorNewErrorWithStacktrace(t *testing.T) {

	var err = NewError("formatter%v%v", 0, 1).Stacktrace()

	assert.Equal(t,
		FmtError{"formatter%v%v", []interface{}{0, 1}},
		err.Data())
	assert.Equal(t, "formatter01", fmt.Sprintf("%v", err))
	assert.Contains(t, fmt.Sprintf("%#v", err), `Data: common.FmtError{format:"formatter%v%v", args:[]interface {}{0, 1}}`)
	assert.Contains(t, fmt.Sprintf("%#v", err), "Stack Trace:\n    0")
}

func TestErrorNewErrorWithTrace(t *testing.T) {

	var err = NewError("formatter%v%v", 0, 1)
	err.Trace(0, "trace %v", 1)
	err.Trace(0, "trace %v", 2)
	err.Trace(0, "trace %v", 3)

	assert.Equal(t,
		FmtError{"formatter%v%v", []interface{}{0, 1}},
		err.Data())
	assert.Equal(t, "formatter01", fmt.Sprintf("%v", err))
	assert.Contains(t, fmt.Sprintf("%#v", err), `Data: common.FmtError{format:"formatter%v%v", args:[]interface {}{0, 1}}`)
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
