package pluginhost

import "testing"

func TestNormalizeLocatorFieldAcceptsCopiedKeyValueForms(t *testing.T) {
	tests := []struct {
		name  string
		field string
		raw   string
		want  string
	}{
		{name: "repo equals", field: "repo", raw: "repo=https://github.com/example/repo.git", want: "https://github.com/example/repo.git"},
		{name: "repo colon", field: "repo", raw: "repo: https://github.com/example/repo.git", want: "https://github.com/example/repo.git"},
		{name: "ref equals", field: "ref", raw: "ref=main", want: "main"},
		{name: "path equals", field: "path", raw: "path=plugins/persona", want: "plugins/persona"},
		{name: "path quoted", field: "path", raw: "`path=\"plugins/persona/\"`", want: "plugins/persona"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NormalizeLocatorField(tt.raw, tt.field); got != tt.want {
				t.Fatalf("NormalizeLocatorField(%q, %q) = %q, want %q", tt.raw, tt.field, got, tt.want)
			}
		})
	}
}
