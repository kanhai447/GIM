package main

import "testing"

func TestGeneratedReaderIsDeterministic(t *testing.T) {
	if first, second := generatedSHA256(1024), generatedSHA256(1024); first != second || first == generatedSHA256(1025) {
		t.Fatalf("unexpected deterministic hashes: %q %q", first, second)
	}
}
