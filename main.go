package main

import (
	"fmt"

	_ "bitbucket.org/ossystems/agent/installmodes/copy"
	"bitbucket.org/ossystems/agent/metadata"
)

func main() {
	obj, _ := metadata.PackageObjectFromJSON([]byte("{ \"mode\": \"copy\" }"))
	fmt.Println(obj)

	obj.Setup()

	InstallUpdate(obj)
}
