package tva

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/stretchr/testify/assert"
)

func TestAutoscaler_FMap(t *testing.T) {
	a := Autoscaler{
		Window: "[window value]",
		GUID:   "[guid value]",
	}

	out, err := template.New("test.tmpl").Funcs(a.FMap()).Parse(`hello {{ guid }} with window {{ window }}`)
	if !assert.Nil(t, err) {
		return
	}
	if !assert.NotNil(t, out) {
		return
	}
	var tpl bytes.Buffer

	err = out.Execute(&tpl, nil)
	if !assert.Nil(t, err) {
		return
	}
	assert.Equal(t, "hello [guid value] with window [window value]", tpl.String())
}
