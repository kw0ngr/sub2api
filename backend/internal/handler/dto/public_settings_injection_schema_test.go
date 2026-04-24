//go:build unit

package dto

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// TestPublicSettingsInjectionPayload_SchemaDoesNotDrift ensures SSR injection
// payload keeps up with the public settings DTO fields used by the frontend.
func TestPublicSettingsInjectionPayload_SchemaDoesNotDrift(t *testing.T) {
	injection := jsonTags(reflect.TypeOf(service.PublicSettingsInjectionPayload{}))
	dtoKeys := jsonTags(reflect.TypeOf(PublicSettings{}))

	dtoOnlyFields := map[string]string{
		"sora_client_enabled": "upstream-only field, not used by this fork runtime",
	}

	var missing []string
	for key := range dtoKeys {
		if _, ok := injection[key]; ok {
			continue
		}
		if _, allowed := dtoOnlyFields[key]; allowed {
			continue
		}
		missing = append(missing, key)
	}
	if len(missing) > 0 {
		t.Fatalf("service.PublicSettingsInjectionPayload is missing JSON fields present on dto.PublicSettings: %s", strings.Join(missing, ", "))
	}
}

func jsonTags(t reflect.Type) map[string]struct{} {
	out := make(map[string]struct{})
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := strings.SplitN(tag, ",", 2)[0]
		if name == "" {
			continue
		}
		out[name] = struct{}{}
	}
	return out
}
