/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/OSSystems/pkg/log"
	"github.com/sirupsen/logrus"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"

	"github.com/updatehub/updatehub/client"
	_ "github.com/updatehub/updatehub/installmodes/copy"
	_ "github.com/updatehub/updatehub/installmodes/flash"
	_ "github.com/updatehub/updatehub/installmodes/imxkobs"
	_ "github.com/updatehub/updatehub/installmodes/raw"
	_ "github.com/updatehub/updatehub/installmodes/tarball"
	_ "github.com/updatehub/updatehub/installmodes/ubifs"
	"github.com/updatehub/updatehub/metadata"
	"github.com/updatehub/updatehub/server"
	"github.com/updatehub/updatehub/updatehub"
	"github.com/updatehub/updatehub/utils"
)

var (
	gitversion = "No version provided"
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

	runtimeSettings := &updatehub.Settings{}
	err = loadSettings(osFs, runtimeSettings, settings.RuntimeSettingsPath)
	if err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
		os.Exit(1)
	}

	settings.PersistentPollingSettings = runtimeSettings.PersistentPollingSettings
	settings.PersistentUpdateSettings = runtimeSettings.PersistentUpdateSettings

	log.Info("starting UpdateHub Agent")
	log.Info("    version: ", gitversion)
	log.Info("    system settings path: ", systemSettingsPath)
	log.Info("    runtime settings path: ", settings.RuntimeSettingsPath)
	log.Info("    firmware metadata path: ", settings.FirmwareMetadataPath)
	log.Info("    state change callback path: ", stateChangeCallbackPath)
	log.Info("    error callback path: ", errorCallbackPath)
	log.Info("    validate callback path: ", validateCallbackPath)
	log.Info("    rollback callback path: ", rollbackCallbackPath)

	log.Debug("settings:\n", settings.ToString())

	fm, err := metadata.NewFirmwareMetadata(settings.FirmwareMetadataPath, osFs, &utils.CmdLine{})
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}

	osFs.MkdirAll(settings.DownloadDir, 0755)

	pubKey, err := readPublicKey(osFs, settings)
	if err != nil {
		log.Fatal(errors.Wrap(err, "failed to read public key"))
	}

	uh := updatehub.NewUpdateHub(gitversion, stateChangeCallbackPath, errorCallbackPath, validateCallbackPath, rollbackCallbackPath, osFs, *fm, pubKey, updatehub.NewIdleState(), settings, client.NewApiClient(settings.ServerAddress))

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

	uh.Start()

	log.Info("UpdateHub Agent started")

	d := updatehub.NewDaemon(uh)
	os.Exit(d.Run())
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

	*structToSaveOn = *s

	return nil
}

func readPublicKey(fs afero.Fs, settings *updatehub.Settings) (*rsa.PublicKey, error) {
	pubKeyPath := path.Join(settings.FirmwareMetadataPath, "key.pub")
	data, err := afero.ReadFile(fs, pubKeyPath)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("Failed to decode PEM")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	return key.(*rsa.PublicKey), nil
}
