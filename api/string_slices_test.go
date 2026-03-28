package main

import "testing"

func TestStringSlicesEqual(t *testing.T) {
	tests := []struct {
		name     string
		a, b     []string
		expected bool
	}{
		{"both empty", []string{}, []string{}, true},
		{"equal single", []string{"a"}, []string{"a"}, true},
		{"equal same order", []string{"a", "b"}, []string{"a", "b"}, true},
		{"equal different order", []string{"en", "fr"}, []string{"fr", "en"}, true},
		{"different lengths", []string{"a"}, []string{"a", "b"}, false},
		{"different values", []string{"a", "b"}, []string{"a", "c"}, false},
		{"both nil", nil, nil, true},
		{"three values shuffled", []string{"c", "a", "b"}, []string{"b", "c", "a"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stringSlicesEqual(tt.a, tt.b)
			if got != tt.expected {
				t.Errorf("stringSlicesEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}
