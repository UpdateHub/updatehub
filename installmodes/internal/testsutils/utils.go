package testsutils

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
)

func SetupCheckRequirementsDir(t *testing.T, binaries []string) string {
	// setup a temp dir
	testPath, err := ioutil.TempDir("", "ubifs-test")
	assert.NoError(t, err)

	// setup the binaries on dir
	for _, binary := range binaries {
		err = ioutil.WriteFile(path.Join(testPath, binary), []byte("dummy_data"), 0777)
		assert.NoError(t, err)
	}

	return testPath
}
