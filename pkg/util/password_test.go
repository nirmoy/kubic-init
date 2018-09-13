package util

import (
	"testing"
)

func TestNewSharedPassword(t *testing.T) {
	password, err := NewSharedPassword("test")
	password.Rand(10)
	if err != nil {
		t.Fatalf("Could not generate a password: %s", err)
	}

	t.Logf("Password generated: %s", password)

}
