package client

import "testing"

// Test files may use route literals.
func TestAboutURL(t *testing.T) {
	if AboutURL() != "/about" {
		t.Fatal("wrong")
	}
}
