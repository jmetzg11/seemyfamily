package main

import (
	"os"
	"testing"

	"seemyfamily.jmetzg11/internal/testutil"
)

// TestMain runs before every test in this package, so no individual test or
// helper has to remember the check.
func TestMain(m *testing.M) {
	testutil.MustBeLocal()
	os.Exit(m.Run())
}

// TestAppMailerCannotSend keeps the suite from emailing real people. Tests are
// run with .env sourced, so GMAIL_USER and GMAIL_PASS may well be set; the test
// app has to carry a client that refuses to send no matter what is in the
// environment.
func TestAppMailerCannotSend(t *testing.T) {
	app := newTestApp(t)

	if app.mailer == nil {
		t.Fatal("test app has no mailer; a handler that sends would panic")
	}

	if app.mailer.Configured() {
		t.Error("test app mailer is configured; the suite could send real email")
	}
}
