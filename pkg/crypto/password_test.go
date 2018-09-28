package crypto

import (
	"testing"
)

func TestNewSharedPassword(t *testing.T) {
	password1 := NewSharedPassword("my-password1", "my-namespace")
	password1.Rand(10)
	t.Logf("Password generated 1: %s = %s", password1.Name, password1)
	if password1.Name != "my-namespace/my-password2" {
		t.Fatalf("Unexpected password name: %s", password2.Name)
	}

	password2 := NewSharedPassword("my-password2", "")
	password2.Rand(10)
	t.Logf("Password generated: %s = %s", password2.Name, password2)

	if password2.Name != "kube-system/my-password2" {
		t.Fatalf("Unexpected password name: %s", password2.Name)
	}
}
