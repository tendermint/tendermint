package gotwilio

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"sort"
)

// GenerateSignature computes the Twilio signature for verifying the
// authenticity of a request.  It is based on the specification at:
// https://www.twilio.com/docs/security#validating-requests
func (twilio *Twilio) GenerateSignature(url string, form url.Values) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString(url)

	keys := make(sort.StringSlice, 0, len(form))
	for k := range form {
		keys = append(keys, k)
	}

	keys.Sort()

	for _, k := range keys {
		buf.WriteString(k)
		for _, v := range form[k] {
			buf.WriteString(v)
		}
	}

	mac := hmac.New(sha1.New, []byte(twilio.AuthToken))
	mac.Write(buf.Bytes())

	var expected bytes.Buffer
	coder := base64.NewEncoder(base64.StdEncoding, &expected)
	_, err := coder.Write(mac.Sum(nil))
	if err != nil {
		return nil, err
	}

	err = coder.Close()
	if err != nil {
		return nil, err
	}

	return expected.Bytes(), nil
}

// CheckRequestSignature checks that the X-Twilio-Signature header on a request
// matches the expected signature defined by the GenerateSignature function.
//
// The baseUrl parameter will be prepended to the request URL. It is useful for
// specifying the protocol and host parts of the server URL hosting your endpoint.
//
// Passing a non-POST request or a request without the X-Twilio-Signature
// header is an error.
func (twilio *Twilio) CheckRequestSignature(r *http.Request, baseURL string) (bool, error) {
	if r.Method != "POST" {
		return false, errors.New("Checking signatures on non-POST requests is not implemented")
	}

	if err := r.ParseForm(); err != nil {
		return false, err
	}

	url := baseURL + r.URL.String()

	expected, err := twilio.GenerateSignature(url, r.PostForm)
	if err != nil {
		return false, err
	}

	actual := r.Header.Get("X-Twilio-Signature")
	if actual == "" {
		return false, errors.New("Request does not have a twilio signature header")
	}

	return hmac.Equal(expected, []byte(actual)), nil
}
