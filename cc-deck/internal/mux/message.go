package mux

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
)

// Message represents a pipe delivery request routed through the mux broker.
type Message struct {
	SessionName string `json:"session_name"`
	PipeName    string `json:"pipe_name"`
	Args        string `json:"args"`
}

// DedupKey returns a truncated SHA-256 hash of the message's identifying fields.
// The key format is sha256(session_name + "|" + pipe_name + "|" + args)[:16] hex chars.
func (m Message) DedupKey() string {
	h := sha256.Sum256([]byte(m.SessionName + "|" + m.PipeName + "|" + m.Args))
	return fmt.Sprintf("%x", h[:8])
}

// MarshalLine returns the message as a JSON line (with trailing newline).
func (m Message) MarshalLine() ([]byte, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// UnmarshalMessage parses a JSON-encoded message.
func UnmarshalMessage(data []byte) (Message, error) {
	var m Message
	err := json.Unmarshal(data, &m)
	return m, err
}
