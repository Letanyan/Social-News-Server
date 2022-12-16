package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"time"

	"github.com/mailgun/mailgun-go/v4"
)

var (
	mxDomainName string
	mxApiKey     string
)

func init() {
	mxDomainName = "new-source.app"
	mxApiKey = "1c417d164a9f5cd062464243cb9f7181-69210cfc-7b30fee1"
}

func MailTemplate(from string, to string, subject string, body bytes.Buffer) {
	mg := mailgun.NewMailgun(mxDomainName, mxApiKey)
	m := mg.NewMessage(from, subject, "", to)
	m.SetHtml(body.String())

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	// Send the message with a 10 second timeout
	_, _, e := mg.Send(ctx, m)
	DidFail(e, "send mail")
}

func MailValidationKey(userId int64, to string, key int32) {
	t, _ := template.ParseFiles("templates/verify.html")
	var body bytes.Buffer
	t.Execute(&body, struct {
		UserId int64
		Key    int32
		Src    string
		Footer template.HTML
	}{
		UserId: userId,
		Key:    key,
		Src:    serverAddr,
		Footer: FSFooter(),
	})
	MailTemplate(fmt.Sprintf("no-reply@%s", mxDomainName), to, "New Source Email Verification", body)
}

func MailPasswordReset(userId int64, to string, key int32) {
	t, _ := template.ParseFiles("templates/reset_password.html")
	var body bytes.Buffer
	t.Execute(&body, struct {
		UserId int64
		Key    int32
		Src    string
		Footer template.HTML
	}{
		UserId: userId,
		Key:    key,
		Src:    serverAddr,
		Footer: FSFooter(),
	})
	MailTemplate(fmt.Sprintf("no-reply@%s", mxDomainName), to, "New Source Password Reset", body)
}
