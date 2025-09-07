package email

import (
	"bytes"
	"database/sql"
	"embed"
	"fmt"
	"sync"

	"github.com/wneessen/go-mail"

	ht "html/template"

	"github.com/gofiber/fiber/v2/log"
	"github.com/joashgobin/boiler/helpers"
)

//go:embed templates/*
var templatesFS embed.FS

type emailData struct {
	Subject string
	Body    string
}

type MailModel struct {
	DB        *sql.DB
	WaitGroup *sync.WaitGroup
}

type MailInterface interface {
	Send(to, bcc, subject string, swaps ...any)
	NotifyAdmin(subject string, swaps ...any)
}

func (m *MailModel) NotifyAdmin(subject string, swaps ...any) {
	body := ""
	if len(swaps) > 1 {
		body = fmt.Sprintf(swaps[0].(string), swaps[1:]...)
	} else {
		body = swaps[0].(string)
	}
	SendEmail(helpers.Getenv("ADMIN_EMAIL"), subject, body, "")
}

func (m *MailModel) Send(to, bcc, subject string, swaps ...any) {
	body := ""
	if len(swaps) > 1 {
		body = fmt.Sprintf(swaps[0].(string), swaps[1:]...)
	} else {
		body = swaps[0].(string)
	}
	SendEmail(to, subject, body, bcc)
}

func SendEmail(to string, subject string, body string, bcc string) {
	helpers.Background(
		func() {
			data := emailData{Subject: subject, Body: body}
			var senderAddr string = helpers.Getenv("MAIL_USER_EMAIL")
			var senderName string = helpers.Getenv("MAIL_USERNAME")
			username := senderAddr
			password := helpers.Getenv("MAIL_PW")
			mailHost := helpers.Getenv("MAIL_HOST")

			message := mail.NewMsg()
			if err := message.FromFormat(senderName, senderAddr); err != nil {
				log.Infof("failed to set 'from' address: %s", senderAddr)
			}
			if err := message.To(to); err != nil {
				log.Infof("failed to set 'to' address: %s", to)
			}
			message.Subject(subject)
			message.SetBodyString(mail.TypeTextPlain, body)
			htmlTmpl, err := ht.New("").Funcs(ht.FuncMap{
				"safeHTML": func(s string) ht.HTML {
					return ht.HTML(s)
				},
			}).ParseFS(templatesFS, "templates/email.html")
			if err != nil {
				log.Infof("could not parse email template: %v", err)
				return
			}
			htmlBody := new(bytes.Buffer)
			err = htmlTmpl.ExecuteTemplate(htmlBody, "htmlBody", data)
			if err != nil {
				log.Infof("could not parse email template: %v", err)
				return
			}
			message.AddAlternativeString(mail.TypeTextHTML, htmlBody.String())
			message.AddBcc(bcc)

			client, err := mail.NewClient(mailHost, mail.WithTLSPortPolicy(mail.TLSMandatory),
				mail.WithSMTPAuth(mail.SMTPAuthPlain), mail.WithUsername(username), mail.WithPassword(password))
			if err != nil {
				log.Infof("failed to create mail client: %s\n", err)
			}

			if err := client.DialAndSend(message); err != nil {
				log.Infof("failed to send mail: %s", err)
			}

			// helpers.WasteTime(5)
			// log.Infof("Sending email\n'%s'\n to: %s", message, to)
		})
}
