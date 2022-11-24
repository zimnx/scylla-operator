package assets

import (
	"fmt"
	"text/template"

	"k8s.io/apimachinery/pkg/runtime"
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
	return obj, string(renderedBytes), err
}
