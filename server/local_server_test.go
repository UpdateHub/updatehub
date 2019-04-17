package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/UpdateHub/updatehub/metadata"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type LocalServerTestSuite struct {
	suite.Suite
	suite.TearDownTestSuite
	server *LocalServer
}

func (suite *LocalServerTestSuite) SetupTest() {
	bytes, err := json.Marshal(metadata.UpdateMetadata{
		ProductUID: "548873c4d4e8e751fdd46c38a3d5a8656cf87bf27a404f346ad58086f627a4ea",
	})
	assert.NoError(suite.T(), err)

	f, err := generateUpdatePackage(string(bytes))
	assert.NoError(suite.T(), err)

	pkg, err := NewUpdatePackage(f)
	assert.NotNil(suite.T(), pkg)
	assert.NoError(suite.T(), err)

	s, err := NewLocalServer(pkg)
	assert.NoError(suite.T(), err)

	suite.server = s
}

func (suite *LocalServerTestSuite) TearDownTest() {
	os.RemoveAll(suite.server.updatePackage.file.Name())
}

func (suite *LocalServerTestSuite) TestProbe() {
	req, err := http.NewRequest("POST", "/probe", nil)
	assert.NoError(suite.T(), err)

	rr := httptest.NewRecorder()
	http.HandlerFunc(suite.server.probe).ServeHTTP(rr, req)

	assert.Equal(suite.T(), http.StatusOK, rr.Code)
}

func (suite *LocalServerTestSuite) TestReport() {
	req, err := http.NewRequest("POST", "/report", strings.NewReader("{}"))
	assert.NoError(suite.T(), err)

	rr := httptest.NewRecorder()
	http.HandlerFunc(suite.server.report).ServeHTTP(rr, req)

	assert.Equal(suite.T(), http.StatusOK, rr.Code)
}

func (suite *LocalServerTestSuite) TestGetObject() {
	req, err := http.NewRequest("GET", "/products/1/packages/1/objects/metadata", nil)
	assert.NoError(suite.T(), err)

	rr := httptest.NewRecorder()

	router := mux.NewRouter()
	router.HandleFunc("/products/{product}/packages/{package}/objects/{object}", suite.server.getObject)
	router.ServeHTTP(rr, req)

	assert.Equal(suite.T(), http.StatusOK, rr.Code)
}

func (suite *LocalServerTestSuite) TestStartAndWaitForAvailable() {
	go suite.server.start()

	assert.True(suite.T(), suite.server.waitForAvailable())
}

func TestLocalServerSuite(t *testing.T) {
	suite.Run(t, new(LocalServerTestSuite))
}
