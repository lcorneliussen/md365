package loop

import "testing"

func TestComponentFormat(t *testing.T) {
	tests := map[string]string{
		"Plan.loop":     "loop",
		"Legacy.FLUID":  "fluid",
		"extensionless": "loop",
		"embedded.page": "loop",
	}
	for name, want := range tests {
		if got := componentFormat(name); got != want {
			t.Errorf("componentFormat(%q) = %q, want %q", name, got, want)
		}
	}
}
