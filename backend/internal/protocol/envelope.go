package protocol

import "encoding/json"

// ClientEnvelope is Client → Server.
type ClientEnvelope struct {
	Type      string          `json:"type"`
	RequestID string          `json:"requestId,omitempty"`
	Payload   json.RawMessage `json:"payload"`
}

// ServerEnvelope is Server → Client.
type ServerEnvelope struct {
	Type    string `json:"type"`
	Seq     int    `json:"seq,omitempty"`
	Payload any    `json:"payload"`
}

func Encode(v any) ([]byte, error) {
	return json.Marshal(v)
}

func MustEncode(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		return []byte(`{"type":"error","payload":{"code":"INTERNAL","message":"encode failed"}}`)
	}
	return b
}
