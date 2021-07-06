package tva

import (
	"fmt"
	"strings"
)

type OptionFunc func(timeline *Timeline) error

// WithDebug sets debugging flag
func WithDebug(debug bool) OptionFunc {
	return func(timeline *Timeline) error {
		timeline.debug = debug
		return nil
	}
}

func WithReload(reload bool) OptionFunc {
	return func(t *Timeline) error {
		t.reload = reload
		return nil
	}
}

func WithTenants(tenants string) OptionFunc {
	var vetted []string
	var isDefault bool
	list := strings.Split(tenants, ",")
	for _, l := range list {
		if l == "default" {
			isDefault = true
			continue
		}
		vetted = append(vetted, l)
	}
	return func(t *Timeline) error {
		t.defaultTenant = isDefault
		if len(vetted) > 0 {
			tenants = strings.Join(vetted, ",")
			t.Selectors = append(t.Selectors, fmt.Sprintf("%s in (%s)", tenantLabel, tenants))
		}
		return nil
	}
}
