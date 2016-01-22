// Forked from github.com/SlyMarbo/gmail
package alert

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net/smtp"
	"path/filepath"
	"regexp"
	"strings"
)

// Convenience function
func SendEmail(subject, body string, tos []string) error {
	email := Compose(subject, body)
	email.From = config.GetString("smtp_user")
	email.ContentType = "text/html; charset=utf-8"
	email.AddRecipients(tos...)
	err := email.Send()
	return err
}

// Email represents a single message, which may contain
// attachments.
type Email struct {
	From        string
	To          []string
	Subject     string
	ContentType string
	Body        string
	Attachments map[string][]byte
}

// Compose begins a new email, filling the subject and body,
// and allocating memory for the list of recipients and the
// attachments.
func Compose(Subject, Body string) *Email {
	out := new(Email)
	out.To = make([]string, 0, 1)
	out.Subject = Subject
	out.Body = Body
	out.Attachments = make(map[string][]byte)
	return out
}

// Attach takes a filename and adds this to the message.
// Note that since only the filename is stored (and not
// its path, for privacy reasons), multiple files in
// different directories but with the same filename and
// extension cannot be sent.
func (e *Email) Attach(Filename string) error {
	b, err := ioutil.ReadFile(Filename)
	if err != nil {
		return err
	}

	_, fname := filepath.Split(Filename)
	e.Attachments[fname] = b
	return nil
}

// AddRecipient adds a single recipient.
func (e *Email) AddRecipient(Recipient string) {
	e.To = append(e.To, Recipient)
}

// AddRecipients adds one or more recipients.
func (e *Email) AddRecipients(Recipients ...string) {
	e.To = append(e.To, Recipients...)
}

// Send sends the email, returning any error encountered.
func (e *Email) Send() error {
	if e.From == "" {
		return errors.New("Error: No sender specified. Please set the Email.From field.")
	}
	if e.To == nil || len(e.To) == 0 {
		return errors.New("Error: No recipient specified. Please set the Email.To field.")
	}

	auth := smtp.PlainAuth(
		"",
		config.GetString("smtp_user"),
		config.GetString("smtp_password"),
		config.GetString("smtp_host"),
	)

	conn, err := smtp.Dial(fmt.Sprintf("%v:%v", config.GetString("smtp_host"), config.GetString("smtp_port")))
	if err != nil {
		return err
	}

	err = conn.StartTLS(&tls.Config{})
	if err != nil {
		return err
	}

	err = conn.Auth(auth)
	if err != nil {
		return err
	}

	err = conn.Mail(e.From)
	if err != nil {
		if strings.Contains(err.Error(), "530 5.5.1") {
			return errors.New("Error: Authentication failure. Your username or password is incorrect.")
		}
		return err
	}

	for _, recipient := range e.To {
		err = conn.Rcpt(recipient)
		if err != nil {
			return err
		}
	}

	wc, err := conn.Data()
	if err != nil {
		return err
	}
	defer wc.Close()
	_, err = wc.Write(e.Bytes())
	if err != nil {
		return err
	}

	return nil
}

func (e *Email) Bytes() []byte {
	buf := bytes.NewBuffer(nil)

	var subject = e.Subject
	subject = regexp.MustCompile("\n+").ReplaceAllString(subject, " ")
	subject = regexp.MustCompile(" +").ReplaceAllString(subject, " ")
	buf.WriteString("Subject: " + subject + "\n")
	buf.WriteString("To: <" + strings.Join(e.To, ">,<") + ">\n")
	buf.WriteString("MIME-Version: 1.0\n")

	// Boundary is used by MIME to separate files.
	boundary := "f46d043c813270fc6b04c2d223da"

	if len(e.Attachments) > 0 {
		buf.WriteString("Content-Type: multipart/mixed; boundary=" + boundary + "\n")
		buf.WriteString("--" + boundary + "\n")
	}

	if e.ContentType == "" {
		e.ContentType = "text/plain; charset=utf-8"
	}
	buf.WriteString(fmt.Sprintf("Content-Type: %s\n\n", e.ContentType))
	buf.WriteString(e.Body)

	if len(e.Attachments) > 0 {
		for k, v := range e.Attachments {
			buf.WriteString("\n\n--" + boundary + "\n")
			buf.WriteString("Content-Type: application/octet-stream\n")
			buf.WriteString("Content-Transfer-Encoding: base64\n")
			buf.WriteString("Content-Disposition: attachment; filename=\"" + k + "\"\n\n")

			b := make([]byte, base64.StdEncoding.EncodedLen(len(v)))
			base64.StdEncoding.Encode(b, v)
			buf.Write(b)
			buf.WriteString("\n--" + boundary)
		}

		buf.WriteString("--")
	}

	return buf.Bytes()
}
