package pluginhost

import (
	"encoding/json"
	"testing"
)

func TestValidateAndNormalizeConfigAppliesDefaultsAndRejectsUnknownFields(t *testing.T) {
	schema, err := ParseConfigSchema(json.RawMessage(`{
  "type": "object",
  "properties": {
    "note": {"type": "string", "default": "hello"},
    "threshold": {"type": "integer"}
  },
  "required": ["threshold"]
}`))
	if err != nil {
		t.Fatalf("ParseConfigSchema: %v", err)
	}

	normalized, err := ValidateAndNormalizeConfig(schema, json.RawMessage(`{"threshold":3}`))
	if err != nil {
		t.Fatalf("ValidateAndNormalizeConfig: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(normalized, &parsed); err != nil {
		t.Fatalf("unmarshal normalized: %v", err)
	}
	if parsed["note"] != "hello" || parsed["threshold"] != float64(3) {
		t.Fatalf("unexpected normalized config: %#v", parsed)
	}

	if _, err := ValidateAndNormalizeConfig(schema, json.RawMessage(`{"threshold":3,"extra":true}`)); err == nil {
		t.Fatal("expected unknown field validation error")
	}
}

func TestParseConfigSchemaRejectsUnsupportedPropertyType(t *testing.T) {
	if _, err := ParseConfigSchema(json.RawMessage(`{
  "type": "object",
  "properties": {
    "tags": {"type": "array"}
  }
}`)); err == nil {
		t.Fatal("expected unsupported property type error")
	}
}
