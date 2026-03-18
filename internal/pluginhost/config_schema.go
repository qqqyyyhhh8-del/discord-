package pluginhost

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"kizuna/pkg/pluginapi"
)

const (
	configTypeObject  = "object"
	configTypeString  = "string"
	configTypeInteger = "integer"
	configTypeNumber  = "number"
	configTypeBoolean = "boolean"
)

type ConfigSchema struct {
	Title       string
	Description string
	Properties  []ConfigProperty
}

type ConfigProperty struct {
	Name        string
	Type        string
	Title       string
	Description string
	Required    bool
	Default     json.RawMessage
}

type rawConfigSchema struct {
	Type        string                       `json:"type"`
	Title       string                       `json:"title"`
	Description string                       `json:"description"`
	Properties  map[string]rawConfigProperty `json:"properties"`
	Required    []string                     `json:"required"`
}

type rawConfigProperty struct {
	Type        string          `json:"type"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Default     json.RawMessage `json:"default"`
}

func ParseConfigSchema(raw json.RawMessage) (ConfigSchema, error) {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return ConfigSchema{}, nil
	}

	var parsed rawConfigSchema
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return ConfigSchema{}, fmt.Errorf("invalid config_schema json: %w", err)
	}
	if strings.TrimSpace(parsed.Type) != configTypeObject {
		return ConfigSchema{}, errors.New("config_schema top-level type must be object")
	}

	requiredSet := make(map[string]struct{}, len(parsed.Required))
	for _, name := range parsed.Required {
		name = strings.TrimSpace(name)
		if name == "" {
			return ConfigSchema{}, errors.New("config_schema required contains empty property name")
		}
		requiredSet[name] = struct{}{}
	}

	names := make([]string, 0, len(parsed.Properties))
	for name := range parsed.Properties {
		names = append(names, name)
	}
	sort.Strings(names)

	schema := ConfigSchema{
		Title:       strings.TrimSpace(parsed.Title),
		Description: strings.TrimSpace(parsed.Description),
		Properties:  make([]ConfigProperty, 0, len(names)),
	}
	for _, name := range names {
		property := parsed.Properties[name]
		name = strings.TrimSpace(name)
		if name == "" {
			return ConfigSchema{}, errors.New("config_schema property name cannot be empty")
		}
		propType := strings.TrimSpace(property.Type)
		switch propType {
		case configTypeString, configTypeInteger, configTypeNumber, configTypeBoolean:
		default:
			return ConfigSchema{}, fmt.Errorf("config_schema property %s has unsupported type %s", name, propType)
		}
		if len(property.Default) > 0 {
			if _, err := normalizeConfigPropertyValue(propType, property.Default); err != nil {
				return ConfigSchema{}, fmt.Errorf("config_schema property %s default is invalid: %w", name, err)
			}
		}
		_, required := requiredSet[name]
		schema.Properties = append(schema.Properties, ConfigProperty{
			Name:        name,
			Type:        propType,
			Title:       strings.TrimSpace(property.Title),
			Description: strings.TrimSpace(property.Description),
			Required:    required,
			Default:     normalizeRawJSON(property.Default),
		})
	}
	for name := range requiredSet {
		if _, ok := parsed.Properties[name]; !ok {
			return ConfigSchema{}, fmt.Errorf("config_schema required property %s is missing from properties", name)
		}
	}
	return schema, nil
}

func ValidateAndNormalizeConfig(schema ConfigSchema, raw json.RawMessage) (json.RawMessage, error) {
	if len(schema.Properties) == 0 {
		raw = bytes.TrimSpace(raw)
		if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
			return nil, nil
		}
		return nil, errors.New("plugin does not declare config_schema")
	}

	raw = bytes.TrimSpace(raw)
	values := map[string]json.RawMessage{}
	if len(raw) > 0 && !bytes.Equal(raw, []byte("null")) {
		var parsed map[string]json.RawMessage
		if err := json.Unmarshal(raw, &parsed); err != nil {
			return nil, fmt.Errorf("config must be a json object: %w", err)
		}
		values = parsed
	}

	known := make(map[string]ConfigProperty, len(schema.Properties))
	for _, property := range schema.Properties {
		known[property.Name] = property
	}
	for name := range values {
		if _, ok := known[name]; !ok {
			return nil, fmt.Errorf("config contains unknown field %s", name)
		}
	}

	normalized := make(map[string]json.RawMessage, len(schema.Properties))
	for _, property := range schema.Properties {
		value, ok := values[property.Name]
		if !ok || len(bytes.TrimSpace(value)) == 0 || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			if len(property.Default) > 0 {
				normalized[property.Name] = append(json.RawMessage(nil), property.Default...)
				continue
			}
			if property.Required {
				return nil, fmt.Errorf("config field %s is required", property.Name)
			}
			continue
		}
		canonical, err := normalizeConfigPropertyValue(property.Type, value)
		if err != nil {
			return nil, fmt.Errorf("config field %s is invalid: %w", property.Name, err)
		}
		normalized[property.Name] = canonical
	}

	if len(normalized) == 0 {
		return nil, nil
	}
	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, err
	}
	return encoded, nil
}

func ConfigSchemaHasFields(raw json.RawMessage) bool {
	schema, err := ParseConfigSchema(raw)
	return err == nil && len(schema.Properties) > 0
}

func normalizeConfigPropertyValue(propertyType string, raw json.RawMessage) (json.RawMessage, error) {
	raw = bytes.TrimSpace(raw)
	switch strings.TrimSpace(propertyType) {
	case configTypeString:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, errors.New("must be a string")
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return encoded, nil
	case configTypeInteger:
		var number json.Number
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&number); err != nil {
			return nil, errors.New("must be an integer")
		}
		if _, err := number.Int64(); err != nil {
			return nil, errors.New("must be an integer")
		}
		return json.RawMessage(number.String()), nil
	case configTypeNumber:
		var number json.Number
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		if err := decoder.Decode(&number); err != nil {
			return nil, errors.New("must be a number")
		}
		return json.RawMessage(number.String()), nil
	case configTypeBoolean:
		var value bool
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, errors.New("must be a boolean")
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		return encoded, nil
	default:
		return nil, fmt.Errorf("unsupported config type %s", propertyType)
	}
}

func validateManifestConfigSchema(manifest pluginapi.Manifest) error {
	if len(manifest.ConfigSchema) == 0 {
		return nil
	}
	_, err := ParseConfigSchema(manifest.ConfigSchema)
	return err
}
