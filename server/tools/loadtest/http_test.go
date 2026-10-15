package main

import "testing"

func TestParseGroupSizes(t *testing.T) {
	values, err := parseGroupSizes("3,5,10", 10)
	if err != nil || len(values) != 3 || values[2] != 10 {
		t.Fatalf("values=%v err=%v", values, err)
	}
	if _, err := parseGroupSizes("3,3", 10); err == nil {
		t.Fatal("expected duplicate size rejection")
	}
}
