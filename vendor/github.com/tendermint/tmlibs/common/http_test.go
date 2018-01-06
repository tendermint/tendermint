package common_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tendermint/tmlibs/common"
)

func TestWriteSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	common.WriteSuccess(w, "foo")
	assert.Equal(t, w.Code, 200, "should get a 200")
}

var blankErrResponse = new(common.ErrorResponse)

func TestWriteError(t *testing.T) {
	tests := [...]struct {
		msg  string
		code int
	}{
		0: {
			msg:  "this is a message",
			code: 419,
		},
	}

	for i, tt := range tests {
		w := httptest.NewRecorder()
		msg := tt.msg

		// First check without a defined code, should send back a 400
		common.WriteError(w, errors.New(msg))
		assert.Equal(t, w.Code, http.StatusBadRequest, "#%d: should get a 400", i)
		blob, err := ioutil.ReadAll(w.Body)
		if err != nil {
			assert.Fail(t, "expecting a successful ioutil.ReadAll", "#%d", i)
			continue
		}

		recv := new(common.ErrorResponse)
		if err := json.Unmarshal(blob, recv); err != nil {
			assert.Fail(t, "expecting a successful json.Unmarshal", "#%d", i)
			continue
		}

		assert.Equal(t, reflect.DeepEqual(recv, blankErrResponse), false, "expecting a non-blank error response")

		// Now test with an error that's .HTTPCode() int conforming

		// Reset w
		w = httptest.NewRecorder()

		common.WriteError(w, common.ErrorWithCode(errors.New("foo"), tt.code))
		assert.Equal(t, w.Code, tt.code, "case #%d", i)
	}
}

type marshalFailer struct{}

var errFooFailed = errors.New("foo failed here")

func (mf *marshalFailer) MarshalJSON() ([]byte, error) {
	return nil, errFooFailed
}

func TestWriteCode(t *testing.T) {
	codes := [...]int{
		0: http.StatusOK,
		1: http.StatusBadRequest,
		2: http.StatusUnauthorized,
		3: http.StatusInternalServerError,
	}

	for i, code := range codes {
		w := httptest.NewRecorder()
		common.WriteCode(w, "foo", code)
		assert.Equal(t, w.Code, code, "#%d", i)

		// Then for the failed JSON marshaling
		w = httptest.NewRecorder()
		common.WriteCode(w, &marshalFailer{}, code)
		wantCode := http.StatusBadRequest
		assert.Equal(t, w.Code, wantCode, "#%d", i)
		assert.True(t, strings.Contains(w.Body.String(), errFooFailed.Error()),
			"#%d: expected %q in the error message", i, errFooFailed)
	}
}

type saver struct {
	Foo int    `json:"foo" validate:"min=10"`
	Bar string `json:"bar"`
}

type rcloser struct {
	closeOnce sync.Once
	body      *bytes.Buffer
	closeChan chan bool
}

var errAlreadyClosed = errors.New("already closed")

func (rc *rcloser) Close() error {
	var err = errAlreadyClosed
	rc.closeOnce.Do(func() {
		err = nil
		rc.closeChan <- true
		close(rc.closeChan)
	})
	return err
}

func (rc *rcloser) Read(b []byte) (int, error) {
	return rc.body.Read(b)
}

var _ io.ReadCloser = (*rcloser)(nil)

func makeReq(strBody string) (*http.Request, <-chan bool) {
	closeChan := make(chan bool, 1)
	buf := new(bytes.Buffer)
	buf.Write([]byte(strBody))
	req := &http.Request{
		Header: make(http.Header),
		Body:   &rcloser{body: buf, closeChan: closeChan},
	}
	return req, closeChan
}

func TestParseRequestJSON(t *testing.T) {
	tests := [...]struct {
		body    string
		wantErr bool
		useNil  bool
	}{
		0: {wantErr: true, body: ``},
		1: {body: `{}`},
		2: {body: `{"foo": 2}`}, // Not that the validate tags don't matter here since we are just parsing
		3: {body: `{"foo": "abcd"}`, wantErr: true},
		4: {useNil: true, wantErr: true},
	}

	for i, tt := range tests {
		req, closeChan := makeReq(tt.body)
		if tt.useNil {
			req.Body = nil
		}
		sav := new(saver)
		err := common.ParseRequestJSON(req, sav)
		if tt.wantErr {
			assert.NotEqual(t, err, nil, "#%d: want non-nil error", i)
			continue
		}
		assert.Equal(t, err, nil, "#%d: want nil error", i)
		wasClosed := <-closeChan
		assert.Equal(t, wasClosed, true, "#%d: should have invoked close", i)
	}
}

func TestFparseJSON(t *testing.T) {
	r1 := strings.NewReader(`{"foo": 1}`)
	sav := new(saver)
	require.Equal(t, common.FparseJSON(r1, sav), nil, "expecting successful parsing")
	r2 := strings.NewReader(`{"bar": "blockchain"}`)
	require.Equal(t, common.FparseJSON(r2, sav), nil, "expecting successful parsing")
	require.Equal(t, reflect.DeepEqual(sav, &saver{Foo: 1, Bar: "blockchain"}), true, "should have parsed both")

	// Now with a nil body
	require.NotEqual(t, nil, common.FparseJSON(nil, sav), "expecting a nil error report")
}

func TestFparseAndValidateJSON(t *testing.T) {
	r1 := strings.NewReader(`{"foo": 1}`)
	sav := new(saver)
	require.NotEqual(t, common.FparseAndValidateJSON(r1, sav), nil, "expecting validation to fail")
	r1 = strings.NewReader(`{"foo": 100}`)
	require.Equal(t, common.FparseJSON(r1, sav), nil, "expecting successful parsing")
	r2 := strings.NewReader(`{"bar": "blockchain"}`)
	require.Equal(t, common.FparseAndValidateJSON(r2, sav), nil, "expecting successful parsing")
	require.Equal(t, reflect.DeepEqual(sav, &saver{Foo: 100, Bar: "blockchain"}), true, "should have parsed both")

	// Now with a nil body
	require.NotEqual(t, nil, common.FparseJSON(nil, sav), "expecting a nil error report")
}

var blankSaver = new(saver)

func TestParseAndValidateRequestJSON(t *testing.T) {
	tests := [...]struct {
		body    string
		wantErr bool
		useNil  bool
	}{
		0: {wantErr: true, body: ``},
		1: {body: `{}`, wantErr: true},         // Here it should fail since Foo doesn't meet the minimum value
		2: {body: `{"foo": 2}`, wantErr: true}, // Here validation should fail
		3: {body: `{"foo": "abcd"}`, wantErr: true},
		4: {useNil: true, wantErr: true},
		5: {body: `{"foo": 100}`}, // Must succeed
	}

	for i, tt := range tests {
		req, closeChan := makeReq(tt.body)
		if tt.useNil {
			req.Body = nil
		}
		sav := new(saver)
		err := common.ParseRequestAndValidateJSON(req, sav)
		if tt.wantErr {
			assert.NotEqual(t, err, nil, "#%d: want non-nil error", i)
			continue
		}

		assert.Equal(t, err, nil, "#%d: want nil error", i)
		assert.False(t, reflect.DeepEqual(blankSaver, sav), "#%d: expecting a set saver", i)

		wasClosed := <-closeChan
		assert.Equal(t, wasClosed, true, "#%d: should have invoked close", i)
	}
}

func TestErrorWithCode(t *testing.T) {
	tests := [...]struct {
		code int
		err  error
	}{
		0: {code: 500, err: errors.New("funky")},
		1: {code: 406, err: errors.New("purist")},
	}

	for i, tt := range tests {
		errRes := common.ErrorWithCode(tt.err, tt.code)
		assert.Equal(t, errRes.Error(), tt.err.Error(), "#%d: expecting the error values to be equal", i)
		assert.Equal(t, errRes.Code, tt.code, "expecting the same status code", i)
		assert.Equal(t, errRes.HTTPCode(), tt.code, "expecting the same status code", i)
	}
}
