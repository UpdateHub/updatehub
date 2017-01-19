package main

import (
	"fmt"

	"bitbucket.org/ossystems/agent/pkg"

	// load plugins
	_ "bitbucket.org/ossystems/agent/plugins/copy"
)

func main() {
	obj, _ := pkg.ObjectFromJSON([]byte("{ \"mode\": \"copy\" }"))
	fmt.Println(obj)

	obj.Setup()

	InstallUpdate(obj)
}
