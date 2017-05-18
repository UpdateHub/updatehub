/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/OSSystems/pkg/log"
	"github.com/Sirupsen/logrus"
	"github.com/UpdateHub/updatehub/server"
	"github.com/spf13/cobra"
)

func main() {
	logger := logrus.New()

	var path string

	log.SetLevel(logrus.WarnLevel)

	cmd := &cobra.Command{
		Use: "updatehub-server [PATH]",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				log.Fatal("You must provide a path")
				os.Exit(1)
			}

			path = args[0]
		},
	}

	if err := cmd.Execute(); err != nil {
		logger.Fatal(cmd)
		os.Exit(1)
	}

	backend, err := server.NewServerBackend(path)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	if err := backend.ParseUpdateMetadata(); err != nil {
		if os.IsNotExist(err) {
			log.Info(fmt.Errorf("updatemetadata.json not found in %s", path))
		} else {
			log.Warn(err)
		}
	}

	d, err := server.NewDaemon(backend)
	if err != nil {
		log.Fatal(fmt.Errorf("%s: %s", path, err))
		os.Exit(1)
	}

	go func() {
		router := server.NewBackendRouter(backend)
		if err := http.ListenAndServe(":8080", router.HTTPRouter); err != nil {
			log.Fatal(err)
		}
	}()

	d.Run()
}
