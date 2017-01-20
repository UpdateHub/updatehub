package metadata

import (
	"encoding/json"

	"bitbucket.org/ossystems/agent/installmodes"
	"bitbucket.org/ossystems/agent/pkg"
)

func PackageObjectFromJSON(bytes []byte) (pkg.Object, error) {
	var v map[string]interface{}

	err := json.Unmarshal(bytes, &v)
	if err != nil {
		return nil, err
	}

	var obj pkg.Object

	o, err := installmodes.GetObject(v["mode"].(string))
	if err == nil {
		obj = o.(pkg.Object)
	} else {
		return nil, err
	}

	json.Unmarshal(bytes, &obj)

	return obj, nil
}
