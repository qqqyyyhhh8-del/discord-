package pluginhost

import "strings"

// NormalizeLocatorField accepts either a raw value or a copied "key=value" form
// from the README / UI examples and returns the canonical field value.
func NormalizeLocatorField(raw, field string) string {
	value := trimLocatorValue(raw)
	field = strings.ToLower(strings.TrimSpace(field))
	if field != "" {
		lowerValue := strings.ToLower(value)
		prefixes := []string{
			field + "=",
			field + " =",
			field + ":",
			field + " :",
			field + "：",
			field + " ：",
		}
		for _, prefix := range prefixes {
			if strings.HasPrefix(lowerValue, prefix) {
				value = trimLocatorValue(value[len(prefix):])
				break
			}
		}
	}
	if field == "path" {
		value = strings.Trim(value, "/")
	}
	return value
}

func trimLocatorValue(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`")
	value = strings.Trim(value, "\"'")
	value = strings.TrimSpace(value)
	value = strings.TrimRight(value, ",，;；")
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "`")
	value = strings.Trim(value, "\"'")
	return strings.TrimSpace(value)
}
