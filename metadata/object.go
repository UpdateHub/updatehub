package metadata

import (
	"encoding/json"
	"errors"
	"reflect"

	"bitbucket.org/ossystems/agent/handlers"
	"bitbucket.org/ossystems/agent/installmodes"
)

// ObjectMetadata contains the common properties of a package's object from JSON metadata
type ObjectMetadata struct {
	Object `json:"-"`

	Sha256sum  string `json:"sha256sum"`
	Mode       string `json:"mode"`
	Compressed bool   `json:"bool"`
}

func NewObjectMetadata(bytes []byte) (Object, error) {
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

	if compressed, ok := v["compressed"].(bool); ok && compressed {
		field, ok := reflect.TypeOf(obj).FieldByName("CompressedObject")

		if !ok || field.Type != reflect.TypeOf(CompressedObject{}) {
			return nil, errors.New("Compressed object does not embed CompressedObject struct")
		}
	}

	return obj, nil
}

func (o ObjectMetadata) GetObjectMetadata() ObjectMetadata {
	return o
}

type CompressedObject struct {
	CompressedSize   float64 `json:"required-compressed-size"`
	UncompressedSize float64 `json:"required-uncompressed-size"`
}

type Object interface {
	handlers.InstallUpdateHandler

	GetObjectMetadata() ObjectMetadata
}
