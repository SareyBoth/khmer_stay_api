package platform

import "testing"

func TestHighestPreference(t *testing.T) {
	scores := map[string]float64{"KH-17": 4, "KH-07": 9, "": 100}
	if got := highestPreference(scores); got != "KH-07" {
		t.Fatalf("highestPreference() = %q, want KH-07", got)
	}
}

func TestHighestPreferenceEmpty(t *testing.T) {
	if got := highestPreference(map[string]float64{}); got != "" {
		t.Fatalf("highestPreference() = %q, want empty", got)
	}
}
