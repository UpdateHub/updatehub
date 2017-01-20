package installmodes

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckRequirements(t *testing.T) {
	RegisterInstallMode("test", InstallMode{
		CheckRequirements: func() error { return nil },
	})

	err := CheckRequirements()
	assert.NoError(t, err)
}

func TestFailCheckRequirements(t *testing.T) {
	RegisterInstallMode("test", InstallMode{
		CheckRequirements: func() error { return errors.New("") },
	})

	err := CheckRequirements()
	assert.Error(t, err)
}
