package mailer

import (
	"errors"
	"fmt"
	"net/smtp"
	"slices"
	"strings"
)

var ErrNotConfigured = errors.New("mailer: not configured")

var sendMail = smtp.SendMail

type Client struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
}

func (c *Client) Configured() bool {
	return c.Username != "" && c.Password != ""
}

func (c *Client) Send(to []string, subject, body string) error {
	if !c.Configured() {
		return ErrNotConfigured
	}

	from := c.From
	if from == "" {
		from = c.Username
	}

	recipients := withSelf(from, to)

	auth := smtp.PlainAuth("", c.Username, c.Password, c.Host)

	err := sendMail(c.Host+":"+c.Port, auth, from, recipients, compose(from, recipients, subject, body))
	if err != nil {
		return fmt.Errorf("mailer: failed to send email: %w", err)
	}
	return nil
}

func withSelf(self string, to []string) []string {
	recipients := []string{self}

	for _, addr := range to {
		addr = strings.TrimSpace(addr)
		if addr == "" || strings.EqualFold(addr, self) {
			continue
		}
		if !slices.ContainsFunc(recipients, func(r string) bool { return strings.EqualFold(r, addr) }) {
			recipients = append(recipients, addr)
		}
	}

	return recipients
}

func compose(from string, to []string, subject, body string) []byte {
	var b strings.Builder

	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", oneLine(subject))
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(strings.ReplaceAll(body, "\n", "\r\n"))
	b.WriteString("\r\n")

	return []byte(b.String())
}

var lineBreaks = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ")

func oneLine(s string) string {
	return strings.TrimSpace(lineBreaks.Replace(s))
}
