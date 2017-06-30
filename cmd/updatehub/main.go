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
	"os"
	"time"

	"github.com/OSSystems/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"

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
	log.SetLevel(logrus.WarnLevel)

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
		API:                       client.NewApiClient("localhost:8080"),
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

	uh.StartPolling()

	d := updatehub.NewDaemon(uh)

	log.Info("UpdateHub Agent started")

	os.Exit(d.Run())
}
