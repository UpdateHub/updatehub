/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/OSSystems/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/UpdateHub/updatehub/client"
	"github.com/UpdateHub/updatehub/installifdifferent"
	_ "github.com/UpdateHub/updatehub/installmodes/copy"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/server"
	"github.com/UpdateHub/updatehub/updatehub"
	"github.com/UpdateHub/updatehub/utils"
)

var (
	gitversion = "No version provided"
	buildtime  = "No build time provided"
)

func main() {
	log.SetLevel(logrus.InfoLevel)

	cmd := &cobra.Command{
		Use: "updatehub",
		Run: func(cmd *cobra.Command, args []string) {
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

	log.Info("starting UpdateHub Agent")
	log.Info("    version: ", gitversion)
	log.Info("    buildtime: ", buildtime)
	log.Info("    system settings path: ", systemSettingsPath)
	log.Info("    runtime settings path: ", runtimeSettingsPath)
	log.Info("    firmware metadata path: ", firmwareMetadataDirPath)

	osFs := afero.NewOsFs()

	fm, err := metadata.NewFirmwareMetadata(firmwareMetadataDirPath, osFs, &utils.CmdLine{})
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	uh := &updatehub.UpdateHub{
		Version:                   gitversion,
		BuildTime:                 buildtime,
		State:                     updatehub.NewIdleState(),
		Updater:                   client.NewUpdateClient(),
		TimeStep:                  time.Minute,
		Store:                     osFs,
		FirmwareMetadata:          *fm,
		SystemSettingsPath:        systemSettingsPath,
		RuntimeSettingsPath:       runtimeSettingsPath,
		Reporter:                  client.NewReportClient(),
		Sha256Checker:             &updatehub.Sha256CheckerImpl{},
		InstallIfDifferentBackend: &installifdifferent.DefaultImpl{FileSystemBackend: osFs},
	}

	backend, err := server.NewAgentBackend(uh, &utils.RebooterImpl{})
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	log.Info("starting API HTTP server")

	go func() {
		router := server.NewBackendRouter(backend)
		if err := http.ListenAndServe(":8080", router.HTTPRouter); err != nil {
			log.Fatal(err)
		} else {
			log.Info("API HTTP server started")
		}
	}()

	uh.Controller = uh

	if err = uh.LoadSettings(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	address, err := sanitizeServerAddress(uh.Settings.ServerAddress)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	uh.API = client.NewApiClient(address)

	uh.StartPolling()

	d := updatehub.NewDaemon(uh)

	log.Info("UpdateHub Agent started")

	os.Exit(d.Run())
}

func sanitizeServerAddress(address string) (string, error) {
	a := address
	if !strings.HasPrefix(a, "http://") && !strings.HasPrefix(a, "https://") {
		a = "https://" + a
	}

	serverURL, err := url.Parse(a)
	if err != nil {
		return "", err
	}

	return serverURL.String(), nil
}
