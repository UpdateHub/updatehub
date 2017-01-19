package pkg

import (
	"encoding/json"

	"bitbucket.org/ossystems/agent/handlers"
	"bitbucket.org/ossystems/agent/plugins"
)

type ObjectData struct {
	Sha256sum string `json:"sha256sum"`
	Mode      string `json:"mode"`
}

func (o ObjectData) Setup() error {
	return nil
}

func (o ObjectData) Install() error {
	return nil
}

func (o ObjectData) Cleanup() error {
	return nil
}

func (o ObjectData) CheckRequirements() error {
	return nil
}

type Object interface {
	handlers.InstallUpdateHandler
}

func ObjectFromJSON(bytes []byte) (Object, error) {
	var v interface{}

	err := json.Unmarshal(bytes, &v)
	if err != nil {
		return nil, err
	}

	var obj Object

	switch v.(map[string]interface{})["mode"] {
	case "copy":
		obj = plugins.GetPlugin("copy").(Object)
	}

	json.Unmarshal(bytes, &obj)

	return obj, nil
}
