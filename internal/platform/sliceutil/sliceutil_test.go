package sliceutil

import "testing"

func TestParseCSV(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"empty string returns nil", "", nil},
		{"whitespace-only string returns nil", "   ", nil},
		{"single value", "foo", []string{"foo"}},
		{"multiple values", "foo,bar,baz", []string{"foo", "bar", "baz"}},
		{"values with surrounding whitespace are trimmed", " foo , bar ", []string{"foo", "bar"}},
		{"empty parts between commas are skipped", "foo,,bar", []string{"foo", "bar"}},
		{"mixed empty and non-empty parts", " , foo, , bar, ", []string{"foo", "bar"}},
		{"single value with trailing comma", "foo,", []string{"foo"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseCSV(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("ParseCSV(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("ParseCSV(%q) = %v, want %v", tt.input, got, tt.want)
				}
			}
		})
	}
}
