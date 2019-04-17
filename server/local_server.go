package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/OSSystems/pkg/log"
	"github.com/UpdateHub/updatehub/libarchive"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type LocalServer struct {
	port          int
	updatePackage *UpdatePackage
}

func NewLocalServer(updatePackage *UpdatePackage) (*LocalServer, error) {
	l := &LocalServer{updatePackage: updatePackage}

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}

	l.port = listener.Addr().(*net.TCPAddr).Port

	defer listener.Close()

	return l, nil
}

func (l *LocalServer) start() error {
	router := mux.NewRouter()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {}).Methods("GET")
	router.HandleFunc("/upgrades", l.probe).Methods("POST")
	router.HandleFunc("/report", l.report).Methods("POST")
	router.HandleFunc("/products/{product}/packages/{package}/objects/{object}", l.getObject).Methods("GET")

	return http.ListenAndServe(fmt.Sprintf("localhost:%d", l.port), router)
}

func (l *LocalServer) waitForAvailable() bool {
	ok := make(chan bool, 1)

	go func() {
		for {
			_, err := http.Get(fmt.Sprintf("http://localhost:%d", l.port))
			if err == nil {
				ok <- true
				break
			}

			time.Sleep(time.Second)
		}
	}()

	select {
	case _ = <-ok:
		ok <- true
	case <-time.After(time.Second * 30):
		ok <- false
	}

	return <-ok
}

func (l *LocalServer) probe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("UH-Signature", string(l.updatePackage.signature))

	if _, err := w.Write(l.updatePackage.updateMetadata); err != nil {
		log.Warn(err)
	}
}

func (l *LocalServer) report(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)

	type reportStruct struct {
		Status       string `json:"status"`
		PackageUID   string `json:"package-uid"`
		ErrorMessage string `json:"error-message"`
	}

	var report reportStruct

	if err := decoder.Decode(&report); err != nil {
		w.WriteHeader(500)
		return
	}

	log.WithFields(logrus.Fields{
		"status":  report.Status,
		"package": report.PackageUID,
		"error":   report.ErrorMessage,
	})
}

func (l *LocalServer) getObject(w http.ResponseWriter, r *http.Request) {
	reader, err := libarchive.NewReader(libarchive.LibArchive{}, l.updatePackage.file.Name(), 10240)
	if err != nil {
		log.Error(err)
		w.WriteHeader(500)
		return
	}

	defer reader.Free()

	err = reader.ExtractFile(mux.Vars(r)["object"], w)
	if err != nil {
		w.WriteHeader(500)
	}
}
