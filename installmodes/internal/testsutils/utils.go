package testsutils

import (
	"fmt"
	"io/ioutil"
	"path"
	"strings"
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

func MergeErrorList(errorList []error) error {
	if len(errorList) == 0 {
		return nil
	}

	if len(errorList) == 1 {
		return errorList[0]
	}

	errorMessages := []string{}
	for _, err := range errorList {
		errorMessages = append(errorMessages, fmt.Sprintf("(%v)", err))
	}

	return fmt.Errorf("%s", strings.Join(errorMessages[:], "; "))
}
