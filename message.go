package wss

import (
	"encoding/json"
)

// Message Message
type Message struct {
	Command string          `json:"command"`
	Channel string          `json:"channel"`
	Data    json.RawMessage `json:"data"`
}

// Reply Reply
type Reply struct {
	Type    string          `json:"type,omitempty"`
	Channel string          `json:"channel,omitempty"`
	Data    json.RawMessage `json:"data"`
}
