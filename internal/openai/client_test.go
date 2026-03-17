package openai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestChatMessageMarshalSupportsMultimodalContent(t *testing.T) {
	message := ChatMessage{
		Role: "user",
		Parts: []ChatContentPart{
			TextPart("hello"),
			ImageURLPart("https://example.com/image.png"),
		},
	}

	data, err := json.Marshal(message)
	if err != nil {
		t.Fatalf("marshal chat message: %v", err)
	}

	payload := string(data)
	if !strings.Contains(payload, `"type":"text"`) {
		t.Fatalf("expected text part in payload, got %s", payload)
	}
	if !strings.Contains(payload, `"type":"image_url"`) {
		t.Fatalf("expected image part in payload, got %s", payload)
	}
	if !strings.Contains(payload, `https://example.com/image.png`) {
		t.Fatalf("expected image url in payload, got %s", payload)
	}
}

func TestChatMessageUnmarshalCollectsTextFromArrayContent(t *testing.T) {
	var message ChatMessage
	data := []byte(`{
		"role": "assistant",
		"content": [
			{"type":"text","text":"第一段"},
			{"type":"image_url","image_url":{"url":"https://example.com/image.png"}},
			{"type":"text","text":"第二段"}
		]
	}`)

	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatalf("unmarshal chat message: %v", err)
	}
	if message.Role != "assistant" {
		t.Fatalf("unexpected role: %q", message.Role)
	}
	if message.Content != "第一段\n第二段" {
		t.Fatalf("unexpected flattened content: %q", message.Content)
	}
	if len(message.Parts) != 3 {
		t.Fatalf("expected 3 parts, got %#v", message.Parts)
	}
}
