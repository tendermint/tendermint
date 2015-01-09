package alert

import (
	"fmt"
	"time"

	"github.com/sfreiberg/gotwilio"
	"github.com/tendermint/tendermint/config"
)

var lastAlertUnix int64 = 0
var alertCountSince int = 0

// Sends a critical alert message to administrators.
func Alert(message string) {
	log.Error("<!> ALERT <!>\n" + message)
	now := time.Now().Unix()
	if now-lastAlertUnix > int64(config.App.GetInt("Alert.MinInterval")) {
		message = fmt.Sprintf("%v:%v", config.App.GetString("Network"), message)
		if alertCountSince > 0 {
			message = fmt.Sprintf("%v (+%v more since)", message, alertCountSince)
			alertCountSince = 0
		}
		if len(config.App.GetString("Alert.TwilioSid")) > 0 {
			go sendTwilio(message)
		}
		if len(config.App.GetString("Alert.EmailRecipients")) > 0 {
			go sendEmail(message)
		}
	} else {
		alertCountSince++
	}
}

func sendTwilio(message string) {
	defer func() {
		if err := recover(); err != nil {
			log.Error("sendTwilio error", "error", err)
		}
	}()
	if len(message) > 50 {
		message = message[:50]
	}
	twilio := gotwilio.NewTwilioClient(config.App.GetString("Alert.TwilioSid"), config.App.GetString("Alert.TwilioToken"))
	res, exp, err := twilio.SendSMS(config.App.GetString("Alert.TwilioFrom"), config.App.GetString("Alert.TwilioTo"), message, "", "")
	if exp != nil || err != nil {
		log.Error("sendTwilio error", "res", res, "exp", exp, "error", err)
	}
}

func sendEmail(message string) {
	defer func() {
		if err := recover(); err != nil {
			log.Error("sendEmail error", "error", err)
		}
	}()
	subject := message
	if len(subject) > 80 {
		subject = subject[:80]
	}
	err := SendEmail(subject, message, config.App.GetStringSlice("Alert.EmailRecipients"))
	if err != nil {
		log.Error("sendEmail error", "error", err, "message", message)
	}
}
