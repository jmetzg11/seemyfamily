package mailer

import (
	"errors"
	"net/smtp"
	"os"
	"slices"
	"strings"
	"testing"
)

type capture struct {
	addr string
	from string
	to   []string
	msg  string
	err  error
}

func stub(t *testing.T, c *capture) {
	t.Helper()

	original := sendMail
	t.Cleanup(func() { sendMail = original })

	sendMail = func(addr string, a smtp.Auth, from string, to []string, msg []byte) error {
		c.addr = addr
		c.from = from
		c.to = to
		c.msg = string(msg)
		return c.err
	}
}

func testClient() *Client {
	return &Client{
		Host:     "smtp.gmail.com",
		Port:     "587",
		Username: "me@gmail.com",
		Password: "app-password",
	}
}

func TestSendNotConfigured(t *testing.T) {
	tests := []struct {
		name   string
		client *Client
	}{
		{"empty", &Client{}},
		{"no username", &Client{Password: "app-password"}},
		{"no password", &Client{Username: "me@gmail.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c capture
			stub(t, &c)

			err := tt.client.Send(nil, "Subject", "Body")
			if !errors.Is(err, ErrNotConfigured) {
				t.Errorf("got %v; want ErrNotConfigured", err)
			}
			if c.addr != "" {
				t.Errorf("sent mail anyway, to %q", c.addr)
			}
		})
	}
}

func TestSendAddressesAndDial(t *testing.T) {
	var c capture
	stub(t, &c)

	err := testClient().Send([]string{"aunt@example.com"}, "Subject", "Body")
	if err != nil {
		t.Fatal(err)
	}

	if c.addr != "smtp.gmail.com:587" {
		t.Errorf("addr: got %q", c.addr)
	}
	if c.from != "me@gmail.com" {
		t.Errorf("from: got %q", c.from)
	}
	if want := []string{"me@gmail.com", "aunt@example.com"}; !slices.Equal(c.to, want) {
		t.Errorf("to: got %v; want %v", c.to, want)
	}
}

func TestSendAlwaysIncludesSelf(t *testing.T) {
	tests := []struct {
		name string
		to   []string
		want []string
	}{
		{"nil", nil, []string{"me@gmail.com"}},
		{"empty", []string{}, []string{"me@gmail.com"}},
		{"blank entry", []string{"", "  "}, []string{"me@gmail.com"}},
		{"others", []string{"a@example.com", "b@example.com"}, []string{"me@gmail.com", "a@example.com", "b@example.com"}},
		{"self repeated", []string{"me@gmail.com"}, []string{"me@gmail.com"}},
		{"self different case", []string{"ME@Gmail.com"}, []string{"me@gmail.com"}},
		{"duplicate other", []string{"a@example.com", "A@example.com"}, []string{"me@gmail.com", "a@example.com"}},
		{"untrimmed", []string{"  a@example.com  "}, []string{"me@gmail.com", "a@example.com"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c capture
			stub(t, &c)

			err := testClient().Send(tt.to, "Subject", "Body")
			if err != nil {
				t.Fatal(err)
			}

			if !slices.Equal(c.to, tt.want) {
				t.Errorf("got %v; want %v", c.to, tt.want)
			}
		})
	}
}

func TestSendUsesFromOverUsername(t *testing.T) {
	var c capture
	stub(t, &c)

	client := testClient()
	client.From = "family@example.com"

	err := client.Send(nil, "Subject", "Body")
	if err != nil {
		t.Fatal(err)
	}

	if c.from != "family@example.com" {
		t.Errorf("from: got %q", c.from)
	}
	if want := []string{"family@example.com"}; !slices.Equal(c.to, want) {
		t.Errorf("to: got %v; want %v", c.to, want)
	}
}

func TestSendWrapsError(t *testing.T) {
	sentinel := errors.New("dial tcp: refused")

	var c capture
	c.err = sentinel
	stub(t, &c)

	err := testClient().Send(nil, "Subject", "Body")
	if !errors.Is(err, sentinel) {
		t.Errorf("got %v; want it to wrap %v", err, sentinel)
	}
}

func TestCompose(t *testing.T) {
	got := string(compose("me@gmail.com", []string{"me@gmail.com", "aunt@example.com"}, "A photo was added", "Line one\nLine two"))

	head, body, found := strings.Cut(got, "\r\n\r\n")
	if !found {
		t.Fatalf("no blank line between headers and body:\n%q", got)
	}

	wantHeaders := []string{
		"From: me@gmail.com",
		"To: me@gmail.com, aunt@example.com",
		"Subject: A photo was added",
		"Content-Type: text/plain; charset=\"utf-8\"",
	}
	if headers := strings.Split(head, "\r\n"); !slices.Equal(headers, wantHeaders) {
		t.Errorf("headers: got %q; want %q", headers, wantHeaders)
	}

	if want := "Line one\r\nLine two\r\n"; body != want {
		t.Errorf("body: got %q; want %q", body, want)
	}

	if strings.Contains(strings.ReplaceAll(got, "\r\n", "\n"), "\r") {
		t.Error("message contains a bare CR")
	}
}

func TestComposeSubjectStaysOnOneLine(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		want    string
	}{
		{"crlf injection", "Hi\r\nBcc: sneak@example.com", "Subject: Hi Bcc: sneak@example.com"},
		{"lf injection", "Hi\nBcc: sneak@example.com", "Subject: Hi Bcc: sneak@example.com"},
		{"bare cr", "Hi\rthere", "Subject: Hi there"},
		{"surrounding space", "  Hi  ", "Subject: Hi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(compose("me@gmail.com", []string{"me@gmail.com"}, tt.subject, "Body"))

			head, _, _ := strings.Cut(got, "\r\n\r\n")
			headers := strings.Split(head, "\r\n")

			if !slices.Contains(headers, tt.want) {
				t.Errorf("got headers %q; want one to be %q", headers, tt.want)
			}
			if len(headers) != 4 {
				t.Errorf("got %d headers, want 4: %q", len(headers), headers)
			}
		})
	}
}

func TestSendLive(t *testing.T) {
	if os.Getenv("MAILER_LIVE") == "" {
		t.Skip("set MAILER_LIVE=1 to send a real email via GMAIL_USER/GMAIL_PASS")
	}

	client := &Client{
		Host:     "smtp.gmail.com",
		Port:     "587",
		Username: os.Getenv("GMAIL_USER"),
		Password: os.Getenv("GMAIL_PASS"),
	}

	if !client.Configured() {
		t.Fatal("GMAIL_USER and GMAIL_PASS must be set")
	}

	err := client.Send(nil, "seemyfamily mailer test", "If you are reading this, the mailer works.")
	if err != nil {
		t.Fatal(err)
	}
}
