package tva

import (
	"github.com/percona/promconfig"
)

type RelabelConfig struct {
	SourceLabels []string `json:"source_labels,omitempty"`
	Separator    string   `json:"separator,omitempty"`
	Regex        string   `json:"regex,omitempty"`
	Modulus      uint64   `json:"modulus,omitempty"`
	TargetLabel  string   `json:"target_label,omitempty"`
	Replacement  string   `json:"replacement,omitempty"`
	Action       string   `json:"action,omitempty"`
}

func (r *RelabelConfig) ToProm() *promconfig.RelabelConfig {
	dest := &promconfig.RelabelConfig{
		SourceLabels: r.SourceLabels,
		Separator:    r.Separator,
		Regex:        r.Regex,
		Modulus:      r.Modulus,
		TargetLabel:  r.TargetLabel,
		Replacement:  r.Replacement,
		Action:       r.Action,
	}
	return dest
}
