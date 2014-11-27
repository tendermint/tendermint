package alert

import (
	"fmt"
	"github.com/sfreiberg/gotwilio"
	"time"

	. "github.com/tendermint/tendermint/config"
)

var last int64 = 0
var count int = 0

func Alert(message string) {
	log.Error("<!> ALERT <!>\n" + message)
	now := time.Now().Unix()
	if now-last > int64(Config.Alert.MinInterval) {
		message = fmt.Sprintf("%v:%v", Config.Network, message)
		if count > 0 {
			message = fmt.Sprintf("%v (+%v more since)", message, count)
			count = 0
		}
		if len(Config.Alert.TwilioSid) > 0 {
			go sendTwilio(message)
		}
		if len(Config.Alert.EmailRecipients) > 0 {
			go sendEmail(message)
		}
	} else {
		count++
	}
}

func sendTwilio(message string) {
	defer func() {
		if err := recover(); err != nil {
			log.Error("sendTwilio error: %v", err)
		}
	}()
	if len(message) > 50 {
		message = message[:50]
	}
	twilio := gotwilio.NewTwilioClient(Config.Alert.TwilioSid, Config.Alert.TwilioToken)
	res, exp, err := twilio.SendSMS(Config.Alert.TwilioFrom, Config.Alert.TwilioTo, message, "", "")
	if exp != nil || err != nil {
		log.Error("sendTwilio error: %v %v %v", res, exp, err)
	}
}

func sendEmail(message string) {
	defer func() {
		if err := recover(); err != nil {
			log.Error("sendEmail error: %v", err)
		}
	}()
	subject := message
	if len(subject) > 80 {
		subject = subject[:80]
	}
	err := SendEmail(subject, message, Config.Alert.EmailRecipients)
	if err != nil {
		log.Error("sendEmail error: %v\n%v", err, message)
	}
}
