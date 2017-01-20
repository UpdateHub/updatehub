package metadata

import "bitbucket.org/ossystems/agent/handlers"

// ObjectData contains the common properties of a package's object from JSON metadata
type ObjectData struct {
	Sha256sum string `json:"sha256sum"`
	Mode      string `json:"mode"`
}

type Object interface {
	handlers.InstallUpdateHandler
}
