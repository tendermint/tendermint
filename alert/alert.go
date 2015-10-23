package alert

import (
	"fmt"
	"time"

	"github.com/sfreiberg/gotwilio"
)

var lastAlertUnix int64 = 0
var alertCountSince int = 0

// Sends a critical alert message to administrators.
func Alert(message string) {
	log.Error("<!> ALERT <!>\n" + message)
	now := time.Now().Unix()
	if now-lastAlertUnix > int64(config.GetInt("alert_min_interval")) {
		message = fmt.Sprintf("%v:%v", config.GetString("chain_id"), message)
		if alertCountSince > 0 {
			message = fmt.Sprintf("%v (+%v more since)", message, alertCountSince)
			alertCountSince = 0
		}
		if len(config.GetString("alert_twilio_sid")) > 0 {
			go sendTwilio(message)
		}
		if len(config.GetString("alert_email_recipients")) > 0 {
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
	twilio := gotwilio.NewTwilioClient(config.GetString("alert_twilio_sid"), config.GetString("alert_twilio_token"))
	res, exp, err := twilio.SendSMS(config.GetString("alert_twilio_from"), config.GetString("alert_twilio_to"), message, "", "")
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
	err := SendEmail(subject, message, config.GetStringSlice("alert_email_recipients"))
	if err != nil {
		log.Error("sendEmail error", "error", err, "message", message)
	}
}
