package utils

import "testing"

func TestParseBoolEnv(t *testing.T) {
	tests := []struct {
		name         string
		value        string
		defaultValue bool
		want         bool
	}{
		{name: "empty false", value: "", defaultValue: false, want: false},
		{name: "empty true default", value: "", defaultValue: true, want: true},
		{name: "false", value: "false", defaultValue: true, want: false},
		{name: "zero", value: "0", defaultValue: true, want: false},
		{name: "true", value: "true", defaultValue: false, want: true},
		{name: "one", value: "1", defaultValue: false, want: true},
		{name: "case insensitive", value: " TRUE ", defaultValue: false, want: true},
		{name: "invalid uses default false", value: "off", defaultValue: false, want: false},
		{name: "invalid uses default true", value: "off", defaultValue: true, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ParseBoolEnv(tt.value, tt.defaultValue); got != tt.want {
				t.Fatalf("ParseBoolEnv(%q, %t) = %t, want %t", tt.value, tt.defaultValue, got, tt.want)
			}
		})
	}
}
