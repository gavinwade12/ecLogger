package main

import "testing"

func TestParseSelection(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"0\n", 0, false},
		{"2\r\n", 2, false},
		{"1", 1, false}, // EOF without a newline
		{"  1  \n", 1, false},
		{"\n", 0, true},
		{"", 0, true},
		{"abc\n", 0, true},
		{"3\n", 0, true},
		{"-1\n", 0, true},
	}
	for _, tt := range tests {
		got, err := parseSelection(tt.input, 3)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseSelection(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if got != tt.want {
			t.Errorf("parseSelection(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}
