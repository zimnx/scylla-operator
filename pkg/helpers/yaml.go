package helpers

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

func EncodeYaml(codecs runtime.Codec, obj runtime.Object) ([]byte, error) {
	jsonBytes, err := runtime.Encode(codecs, obj)
	if err != nil {
		return nil, fmt.Errorf("can't encode object: %q", err)
	}

	yamlBytes, err := yaml.JSONToYAML(jsonBytes)
	if err != nil {
		return nil, fmt.Errorf("can't convert json to yaml: %q", err)
	}

	return yamlBytes, nil
}
