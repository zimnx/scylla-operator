package assets

import (
	"fmt"
	"text/template"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

func Decode[T any](data []byte, decoder runtime.Decoder) (T, error) {
	obj, _, err := decoder.Decode(data, nil, nil)
	if err != nil {
		return *new(T), fmt.Errorf("can't decode object: %w", err)
	}

	typedObj, ok := obj.(T)
	if !ok {
		return *new(T), fmt.Errorf("can't cast decoded object: %w", err)
	}

	return typedObj, nil
}

func RenderAndDecode[T any](tmpl *template.Template, inputs any, decoder runtime.Decoder) (T, string, error) {
	renderedBytes, err := RenderTemplate(tmpl, inputs)
	if err != nil {
		return *new(T), "", fmt.Errorf("can't render template: %w", err)
	}

	obj, err := Decode[T](renderedBytes, decoder)
	if err != nil {
		klog.Errorf("Can't decode rendered template %q: %w. Template:\n%s", tmpl.Name(), err, string(renderedBytes))
		return *new(T), string(renderedBytes), fmt.Errorf("can't decode rendered template %q: %w", tmpl.Name(), err)
	}

	return obj, string(renderedBytes), nil
}
