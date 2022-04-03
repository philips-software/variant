package tva

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"text/template"
	"time"
)

type Autoscaler struct {
	Min        int    `json:"min"`
	Max        int    `json:"max"`
	Expression string `json:"expr"`
	Query      string `json:"query"`
	Window     string `json:"window"`
	GUID       string `json:"-"`
}

type State struct {
	Current  int       `json:"-"`
	Want     int       `json:"-"`
	Cooldown int       `json:"-"`
	LastEval time.Time `json:"-"`
}

func (a Autoscaler) Hash() string {
	h := sha1.New()
	input := fmt.Sprintf("%d%d%s%s%s%s", a.Max, a.Max, a.Expression, a.Query, a.Window, a.GUID)
	_, _ = h.Write([]byte(input))
	bs := h.Sum(nil)
	return fmt.Sprintf("%x", bs)
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
