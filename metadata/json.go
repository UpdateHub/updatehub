package metadata

import (
	"encoding/json"

	"bitbucket.org/ossystems/agent/installmodes"
)

func FromJSON(bytes []byte) (*Metadata, error) {
	var wrapper struct {
		Metadata
		RawObjects [][]interface{} `json:"objects"`
	}

	err := json.Unmarshal(bytes, &wrapper)
	if err != nil {
		return nil, err
	}

	// Unwraps metadata
	metadata := wrapper.Metadata

	for _, list := range wrapper.RawObjects {
		var objects []Object

		for _, obj := range list {
			// It is safe to ignore errors here
			b, _ := json.Marshal(obj)

			o, err := ObjectFromJSON(b)
			if err != nil {
				return nil, err
			}

			objects = append(objects, o)
		}

		metadata.Objects = append(metadata.Objects, objects)
	}

	return &metadata, nil
}

func ObjectFromJSON(bytes []byte) (Object, error) {
	var v map[string]interface{}

	err := json.Unmarshal(bytes, &v)
	if err != nil {
		return nil, err
	}

	var obj Object

	o, err := installmodes.GetObject(v["mode"].(string))
	if err == nil {
		obj = o.(Object)
	} else {
		return nil, err
	}

	json.Unmarshal(bytes, &obj)

	return obj, nil
}
