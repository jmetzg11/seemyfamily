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
