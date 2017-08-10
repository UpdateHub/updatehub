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
	"path/filepath"
	"strings"
	"time"

	"github.com/OSSystems/pkg/log"
	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/UpdateHub/updatehub/client"
	_ "github.com/UpdateHub/updatehub/installmodes/copy"
	_ "github.com/UpdateHub/updatehub/installmodes/flash"
	_ "github.com/UpdateHub/updatehub/installmodes/imxkobs"
	_ "github.com/UpdateHub/updatehub/installmodes/raw"
	_ "github.com/UpdateHub/updatehub/installmodes/tarball"
	_ "github.com/UpdateHub/updatehub/installmodes/ubifs"
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

	osFs := afero.NewOsFs()
	settings := &updatehub.Settings{}

	err = loadSettings(osFs, settings, systemSettingsPath)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	err = loadSettings(osFs, settings, settings.RuntimeSettingsPath)
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
		os.Exit(1)
	}

	log.Info("starting UpdateHub Agent")
	log.Info("    version: ", gitversion)
	log.Info("    buildtime: ", buildtime)
	log.Info("    system settings path: ", systemSettingsPath)
	log.Info("    runtime settings path: ", settings.RuntimeSettingsPath)
	log.Info("    firmware metadata path: ", settings.FirmwareMetadataPath)

	log.Debug("settings:\n", settings.ToString())

	fm, err := metadata.NewFirmwareMetadata(settings.FirmwareMetadataPath, osFs, &utils.CmdLine{})
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	osFs.MkdirAll(settings.DownloadDir, 0755)

	uh := updatehub.NewUpdateHub(gitversion, buildtime, osFs, *fm, updatehub.NewIdleState(), settings)

	backend, err := server.NewAgentBackend(uh)
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

	address, err := sanitizeServerAddress(uh.Settings.ServerAddress)
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	uh.API = client.NewApiClient(address)

	uh.StartPolling()

	log.Info("UpdateHub Agent started")

	if !uh.Settings.ManualMode {
		d := updatehub.NewDaemon(uh)
		os.Exit(d.Run())
	} else {
		for {
			time.Sleep(time.Second)
		}
	}
}

func loadSettings(fs afero.Fs, structToSaveOn *updatehub.Settings, pathToLoadFrom string) error {
	fs.MkdirAll(filepath.Dir(pathToLoadFrom), 0755)

	file, err := fs.Open(pathToLoadFrom)
	if err != nil {
		return err
	}
	defer file.Close()

	s, err := updatehub.LoadSettings(file)
	if err != nil {
		return err
	}

	err = mergo.Merge(structToSaveOn, s)
	if err != nil {
		return err
	}

	return nil
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
