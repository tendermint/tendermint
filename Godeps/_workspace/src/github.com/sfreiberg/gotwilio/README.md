## Overview
This is the start of a library for [Twilio](http://www.twilio.com/). Gotwilio supports making voice calls and sending text messages.

## License
Gotwilio is licensed under a BSD license.

## Installation
To install gotwilio, simply run `go get github.com/sfreiberg/gotwilio`.

## SMS Example

	package main

	import (
		"github.com/sfreiberg/gotwilio"
	)

	func main() {
		accountSid := "ABC123..........ABC123"
		authToken := "ABC123..........ABC123"
		twilio := gotwilio.NewTwilioClient(accountSid, authToken)

		from := "+15555555555"
		to := "+15555555555"
		message := "Welcome to gotwilio!"
		twilio.SendSMS(from, to, message, "", "")
	}

## MMS Example

	package main

	import (
		"github.com/sfreiberg/gotwilio"
	)

	func main() {
		accountSid := "ABC123..........ABC123"
		authToken := "ABC123..........ABC123"
		twilio := gotwilio.NewTwilioClient(accountSid, authToken)

		from := "+15555555555"
		to := "+15555555555"
		message := "Welcome to gotwilio!"
		twilio.SendMMS(from, to, message, "http://host/myimage.gif", "", "")
	}

## Voice Example

	package main

	import (
		"github.com/sfreiberg/gotwilio"
	)

	func main() {
		accountSid := "ABC123..........ABC123"
		authToken := "ABC123..........ABC123"
		twilio := gotwilio.NewTwilioClient(accountSid, authToken)

		from := "+15555555555"
		to := "+15555555555"
		callbackParams := gotwilio.NewCallbackParameters("http://example.com")
		twilio.CallWithUrlCallbacks(from, to, callbackParams)
	}
