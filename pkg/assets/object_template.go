package assets

import (
	"fmt"
	"text/template"

	"github.com/scylladb/scylla-operator/pkg/helpers"
	"k8s.io/apimachinery/pkg/runtime"
)

type ObjectTemplate[T runtime.Object] struct {
	tmpl    *template.Template
	decoder runtime.Decoder
}

func ParseObjectTemplate[T runtime.Object](name, tmplString string, funcMap template.FuncMap, decoder runtime.Decoder) (ObjectTemplate[T], error) {
	tmpl, err := template.New(name).Funcs(funcMap).Parse(tmplString)
	if err != nil {
		return *new(ObjectTemplate[T]), fmt.Errorf("can't parse template %q: %w", name, err)
	}

	return ObjectTemplate[T]{
		tmpl:    tmpl,
		decoder: decoder,
	}, nil
}

func ParseObjectTemplateOrDie[T runtime.Object](name, tmplString string, funcMap template.FuncMap, decoder runtime.Decoder) ObjectTemplate[T] {
	return helpers.Must(ParseObjectTemplate[T](name, tmplString, funcMap, decoder))
}

func (t *ObjectTemplate[T]) RenderObject(inputs any) (T, string, error) {
	obj, s, err := RenderAndDecode[T](t.tmpl, inputs, t.decoder)
	if err != nil {
		return obj, s, fmt.Errorf("can't render and decode template %q: %w", t.tmpl.Name(), err)
	}

	return obj, s, nil
}
