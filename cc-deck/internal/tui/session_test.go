package tui

import (
	"encoding/json"
	"testing"
)

func TestParseActivity_SimpleVariants(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{`"Init"`, "Init"},
		{`"Working"`, "Working"},
		{`"Idle"`, "Idle"},
		{`"Done"`, "Done"},
		{`"AgentDone"`, "AgentDone"},
	}
	for _, tc := range cases {
		got := parseActivity(json.RawMessage(tc.input))
		if got != tc.expected {
			t.Errorf("parseActivity(%s) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestParseActivity_WaitingVariants(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{`{"Waiting":"Permission"}`, "Permission"},
		{`{"Waiting":"Notification"}`, "Notification"},
	}
	for _, tc := range cases {
		got := parseActivity(json.RawMessage(tc.input))
		if got != tc.expected {
			t.Errorf("parseActivity(%s) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestParseActivity_InvalidJSON(t *testing.T) {
	got := parseActivity(json.RawMessage(`invalid`))
	if got != "Unknown" {
		t.Errorf("parseActivity(invalid) = %q, want %q", got, "Unknown")
	}
}
