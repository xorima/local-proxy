package adapters_test

import (
	"testing"

	"local-proxy/internal/domains/proxy/adapters"
)

func TestHeaderModifier_Modify(t *testing.T) {
	t.Run("it should set headers", func(t *testing.T) {
		m := adapters.NewHeaderModifier(map[string]string{"X-Custom": "value"}, nil)
		headers := map[string][]string{}
		m.Modify(headers)
		if v := headers["X-Custom"]; len(v) != 1 || v[0] != "value" {
			t.Errorf("got %v, want [value]", v)
		}
	})

	t.Run("it should remove headers", func(t *testing.T) {
		m := adapters.NewHeaderModifier(nil, []string{"X-Remove"})
		headers := map[string][]string{"X-Remove": {"x"}, "X-Keep": {"y"}}
		m.Modify(headers)
		if _, ok := headers["X-Remove"]; ok {
			t.Error("expected X-Remove to be removed")
		}
		if _, ok := headers["X-Keep"]; !ok {
			t.Error("expected X-Keep to remain")
		}
	})

	t.Run("it should override existing headers", func(t *testing.T) {
		m := adapters.NewHeaderModifier(map[string]string{"X-Custom": "new"}, nil)
		headers := map[string][]string{"X-Custom": {"old"}}
		m.Modify(headers)
		if v := headers["X-Custom"]; len(v) != 1 || v[0] != "new" {
			t.Errorf("got %v, want [new]", v)
		}
	})

	t.Run("it should be a no-op with empty config", func(t *testing.T) {
		m := adapters.NewHeaderModifier(nil, nil)
		headers := map[string][]string{"X-Keep": {"y"}}
		m.Modify(headers)
		if v := headers["X-Keep"]; len(v) != 1 || v[0] != "y" {
			t.Errorf("got %v, want [y]", v)
		}
	})
}
