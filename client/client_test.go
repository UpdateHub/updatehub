package client

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewApiClient(t *testing.T) {
	c := NewApiClient("localhost")
	assert.NotNil(t, c)
	assert.Equal(t, "localhost", c.server)
}

func TestApiClientRequest(t *testing.T) {
	c := NewApiClient("localhost")
	assert.NotNil(t, c)

	req := c.Request()
	assert.NotNil(t, req)

	assert.Equal(t, c, req.Client())

	responder := &struct {
		httpStatus int
		headers    http.Header
	}{
		http.StatusOK,
		http.Header{},
	}

	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		responder.headers = r.Header
		w.WriteHeader(responder.httpStatus)
		w.Header().Set("Content-Type", "application/json")
	}))

	defer s.Close()

	hreq, _ := http.NewRequest(http.MethodGet, s.URL, nil)

	res, err := req.Do(hreq)
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.NotNil(t, responder.headers)
	assert.Equal(t, responder.httpStatus, res.StatusCode)
}

func TestServerURL(t *testing.T) {
	c := NewApiClient("localhost")

	url := serverURL(c, "/test")

	assert.Equal(t, "http://localhost/test", url)
}
