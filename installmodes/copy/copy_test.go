package copy

import (
	"testing"

	"bitbucket.org/ossystems/agent/installmodes"
	"github.com/stretchr/testify/assert"
)

func TestIsRegistered(t *testing.T) {
	obj, err := installmodes.GetObject("copy")
	if !assert.NotNil(t, obj) {
		t.Fatal(err)
	}
}
