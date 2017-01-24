package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewApiClient(t *testing.T) {
	c := NewApiClient("localhost")
	assert.NotNil(t, c)
	assert.Equal(t, "localhost", c.server)
}

func TestServerURL(t *testing.T) {
	c := NewApiClient("localhost")

	url := serverURL(c, "/test")

	assert.Equal(t, "http://localhost/test", url)
}
