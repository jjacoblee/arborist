package cli

import (
	"reflect"
	"testing"
)

func TestParseRepoFlag(t *testing.T) {
	tests := []struct {
		name  string
		given []string
		want  []string
	}{
		{"repeated flag", []string{"api", "web"}, []string{"api", "web"}},
		{"comma separated", []string{"api,web,infra"}, []string{"api", "web", "infra"}},
		{"quoted spaces", []string{"api web infra"}, []string{"api", "web", "infra"}},
		{"mixed", []string{"api, web", "infra"}, []string{"api", "web", "infra"}},
		{"owner qualified", []string{"acme/api"}, []string{"acme/api"}},
		{"stray separators", []string{" api ,, web  "}, []string{"api", "web"}},
		{"duplicates collapse", []string{"api", "api,web"}, []string{"api", "web"}},
		{"none", nil, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseRepoFlag(tt.given); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("parseRepoFlag(%q) = %q, want %q", tt.given, got, tt.want)
			}
		})
	}
}
