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
	_ "github.com/UpdateHub/updatehub/installmodes/copy"
	_ "github.com/UpdateHub/updatehub/installmodes/flash"
	_ "github.com/UpdateHub/updatehub/installmodes/imxkobs"
	_ "github.com/UpdateHub/updatehub/installmodes/raw"
	_ "github.com/UpdateHub/updatehub/installmodes/tarball"
	_ "github.com/UpdateHub/updatehub/installmodes/ubifs"
	"github.com/UpdateHub/updatehub/libarchive"
	"github.com/UpdateHub/updatehub/server"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func main() {
	var path string

	log.SetLevel(logrus.InfoLevel)

	cmd := &cobra.Command{
		Use: "updatehub-server path",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				log.Fatal("You must provide a path")
				os.Exit(1)
			}

			path = args[0]
		},
	}

	isQuiet := cmd.PersistentFlags().Bool("quiet", false, "sets the log level to 'error'")
	isDebug := cmd.PersistentFlags().Bool("debug", false, "sets the log level to 'debug'")

	err := cmd.Execute()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	helpCalled, err := cmd.Flags().GetBool("help")
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	if helpCalled {
		os.Exit(1)
	}

	if *isQuiet {
		log.SetLevel(logrus.ErrorLevel)
	}

	if *isDebug {
		log.SetLevel(logrus.DebugLevel)
	}

	la := &libarchive.LibArchive{}

	backend, err := server.NewServerBackend(la, path)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	err = backend.ProcessDirectory()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
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
