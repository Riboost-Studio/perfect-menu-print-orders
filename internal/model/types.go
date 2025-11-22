package model

import "encoding/json"

type MessageType string

const (
	MessageTypeRegister    MessageType = "register"
	MessageTypeRegistered    MessageType = "registered"
	MessageTypeUnregister  MessageType = "unregister"
	MessageTypePing        MessageType = "ping"
	MessageTypePong        MessageType = "pong"
	MessageTypeNewOrder    MessageType = "print_order"
	MessageTypePrinted     MessageType = "printed"
	MessageTypePrintFailed MessageType = "print_failed"
)

// --- WebSocket Messages ---

type WSMessage struct {
	Type     MessageType     `json:"type"`
	AgentKey string          `json:"agent_key,omitempty"`
	Order    json.RawMessage `json:"order,omitempty"` // Keep raw to parse into specific structs
	Error    string          `json:"error,omitempty"`
}
