package metadata

import (
	"encoding/json"

	"bitbucket.org/ossystems/agent/pkg"
	"bitbucket.org/ossystems/agent/plugins"
)

func PackageObjectFromJSON(bytes []byte) (pkg.Object, error) {
	var v interface{}

	err := json.Unmarshal(bytes, &v)
	if err != nil {
		return nil, err
	}

	var obj pkg.Object

	for mode, _ := range plugins.Plugins {
		p := plugins.GetPlugin(mode).(pkg.Object)
		if p != nil {
			obj = p
		}
	}

	json.Unmarshal(bytes, &obj)

	return obj, nil
}
