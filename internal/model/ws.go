package model

import "encoding/json"

// --- WebSocket Messages ---

type WSMessage struct {
	Type     string          `json:"type"`
	AgentKey string          `json:"agent_key,omitempty"`
	Order    json.RawMessage `json:"order,omitempty"` // Keep raw to parse into specific structs
}