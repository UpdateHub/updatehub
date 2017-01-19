package main

import (
	"fmt"

	"bitbucket.org/ossystems/agent/metadata"
	_ "bitbucket.org/ossystems/agent/plugins/copy"
)

func main() {
	obj, _ := metadata.PackageObjectFromJSON([]byte("{ \"mode\": \"copy\" }"))
	fmt.Println(obj)

	obj.Setup()

	InstallUpdate(obj)
}
