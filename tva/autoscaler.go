package tva

import (
	"bytes"
	"text/template"
	"time"
)

type Autoscaler struct {
	Min        int       `json:"min"`
	Max        int       `json:"max"`
	Current    int       `json:"current,omitempty"`
	Expression string    `json:"expr"`
	Query      string    `json:"query"`
	Window     string    `json:"window"`
	LastEval   time.Time `json:"-"`
	GUID       string    `json:"-"`
}

func (a Autoscaler) FMap() template.FuncMap {
	return template.FuncMap{
		"guid": func() string {
			return a.GUID
		},
		"window": func() string {
			return a.Window
		},
	}
}
func (a Autoscaler) RenderQuery() (string, error) {
	t, err := template.New("render.tmpl").Funcs(a.FMap()).Parse(a.Query)
	if err != nil {
		return "", err
	}
	var tpl bytes.Buffer
	err = t.Execute(&tpl, nil)
	return tpl.String(), err
}
