package gotwilio

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/url"
	"testing"
)

const (
	// Magic strings from https://github.com/twilio/twilio-python
	testServerURL      = "http://www.postbin.org"
	testAuthToken      = "1c892n40nd03kdnc0112slzkl3091j20"
	testValidSignature = "fF+xx6dTinOaCdZ0aIeNkHr/ZAA="
)

func TestCheckSignature(t *testing.T) {
	twilio := Twilio{
		AuthToken: testAuthToken,
	}

	// Magic strings from https://github.com/twilio/twilio-python
	u, err := url.Parse("/1ed898x")
	if err != nil {
		t.Fatal(err)
	}
	h := http.Header{
		"Content-Type":       []string{"application/x-www-form-urlencoded"},
		"X-Twilio-Signature": []string{testValidSignature},
	}
	b := bytes.NewBufferString(`FromZip=89449&From=%2B15306666666&` +
		`FromCity=SOUTH+LAKE+TAHOE&ApiVersion=2010-04-01&To=%2B15306384866&` +
		`CallStatus=ringing&CalledState=CA&FromState=CA&Direction=inbound&` +
		`ToCity=OAKLAND&ToZip=94612&CallerCity=SOUTH+LAKE+TAHOE&FromCountry=US&` +
		`CallerName=CA+Wireless+Call&CalledCity=OAKLAND&CalledCountry=US&` +
		`Caller=%2B15306666666&CallerZip=89449&AccountSid=AC9a9f9392lad99kla0sklakjs90j092j3&` +
		`Called=%2B15306384866&CallerCountry=US&CalledZip=94612&CallSid=CAd800bb12c0426a7ea4230e492fef2a4f&` +
		`CallerState=CA&ToCountry=US&ToState=CA`)

	r := http.Request{
		Method: "POST",
		URL:    u,
		Header: h,
		Body:   ioutil.NopCloser(b),
	}

	valid, err := twilio.CheckRequestSignature(&r, testServerURL)
	if err != nil {
		t.Fatal(err)
	}
	if !valid {
		t.Fatal("Expected signature to be valid")
	}

	h["X-Twilio-Signature"] = []string{"foo"}
	valid, err = twilio.CheckRequestSignature(&r, testServerURL)
	if err != nil {
		t.Fatal(err)
	}
	if valid {
		t.Fatal("Expected signature to be invalid")
	}

	delete(h, "X-Twilio-Signature")
	valid, err = twilio.CheckRequestSignature(&r, testServerURL)
	if err == nil {
		t.Fatal("Expected an error verifying a request without a signature header")
	}
}
