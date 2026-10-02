package graph

import "testing"

func TestEscapeDrivePath(t *testing.T) {
	got := escapeDrivePath("/Shared Documents/Project #1/")
	want := "Shared%20Documents/Project%20%231"
	if got != want {
		t.Fatalf("escapeDrivePath() = %q, want %q", got, want)
	}
}
