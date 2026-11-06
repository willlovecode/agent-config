package scan

import "testing"

func TestMemsizeGB(t *testing.T) {
	for _, c := range []struct {
		in   string
		want float64
	}{
		{"17179869184\n", 16},
		{"  8589934592 ", 8},
		{"", 0},
		{"unknown\n", 0},
	} {
		if got := memsizeGB(c.in); got != c.want {
			t.Errorf("memsizeGB(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestProgramsDetectAntigravityAsGemini(t *testing.T) {
	var agy *Program
	for _, p := range programs() {
		if p.Name == "agy" {
			agy = p
		}
	}
	if agy == nil || agy.Fact != "gemini" {
		t.Fatalf("agy must be looked up and count as the gemini tool: %+v", agy)
	}
}
