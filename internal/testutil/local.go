// Package testutil holds helpers shared by the test suites.
package testutil

import (
	"fmt"
	"net/url"
	"os"
)

// localHosts are the only hosts the tests are allowed to touch. The suites
// create and delete rows and bucket objects, so running them against a remote
// host destroys real data.
var localHosts = map[string]bool{
	"localhost": true,
	"127.0.0.1": true,
	"::1":       true,
	"db":        true, // docker compose service name
	"storage":   true, // docker compose service name
}

// CheckLocal returns an error unless rawURL points at a local service. kind
// names the thing being checked, for the error message.
func CheckLocal(kind, rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%s: cannot parse URL: %w", kind, err)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("%s: no host in URL", kind)
	}

	if !localHosts[host] {
		return fmt.Errorf(
			"refusing to run tests against non-local %s host %q: the suite creates and "+
				"deletes data. Source .env (local), not .env.prod",
			kind, host)
	}

	return nil
}

// MustBeLocal aborts the whole test binary unless every backend configured in
// the environment is local. Call it from TestMain so it covers every test in
// the package automatically, including ones written later.
func MustBeLocal() {
	for _, check := range []struct{ kind, env string }{
		{"database", "DATABASE_URL"},
		{"bucket", "S3_ENDPOINT"},
	} {
		raw := os.Getenv(check.env)
		if raw == "" {
			continue
		}

		if err := CheckLocal(check.kind, raw); err != nil {
			fmt.Fprintf(os.Stderr, "\n%v\n\n", err)
			os.Exit(1)
		}
	}
}
