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
	"time"

	"github.com/OSSystems/pkg/log"
	_ "github.com/updatehub/updatehub/installmodes/copy"
	_ "github.com/updatehub/updatehub/installmodes/flash"
	_ "github.com/updatehub/updatehub/installmodes/imxkobs"
	_ "github.com/updatehub/updatehub/installmodes/raw"
	_ "github.com/updatehub/updatehub/installmodes/tarball"
	_ "github.com/updatehub/updatehub/installmodes/ubifs"
	"github.com/updatehub/updatehub/libarchive"
	"github.com/updatehub/updatehub/metadata"
	"github.com/updatehub/updatehub/server"
	"github.com/updatehub/updatehub/updatehub"
	"github.com/parnurzeal/gorequest"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

type AgentInfo struct {
	Version   string                    `json:"version"`
	BuildTime string                    `json:"build-time"`
	Config    updatehub.Settings        `json:"config"`
	Firmware  metadata.FirmwareMetadata `json:"firmware"`
}

type ProbeResponse struct {
	UpdateAvailable bool `json:"update-available"`
	TryAgainIn      int  `json:"try-again-in"`
}

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
	wait := cmd.PersistentFlags().Bool("wait", false, "wait for UpdateHub Agent")

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

	d, err := server.NewDaemon(backend)
	if err != nil {
		log.Fatal(fmt.Errorf("%s: %s", path, err))
		os.Exit(1)
	}

	go func() {
		router := server.NewBackendRouter(backend)
		if err := http.ListenAndServe(":8088", router.HTTPRouter); err != nil {
			log.Fatal(err)
		}
	}()

	var info *AgentInfo

	// Wait and check for UpdateHub Agent is running
	for ok := true; ok; ok = *wait {
		_, _, errs := gorequest.New().Get(buildURL("/info")).EndStruct(&info)
		if len(errs) == 0 {
			log.Info("UpdateHub Agent is running")
			*wait = false
		}

		time.Sleep(time.Second)
	}

	// UpdateHub Agent is running?
	if info != nil {
		probe := ProbeResponse{UpdateAvailable: false}

		var req struct {
			ServerAddress string `json:"server-address"`
		}
		req.ServerAddress = "http://localhost:8088"

		// Probe for update
		_, _, errs := gorequest.New().Post(buildURL("/probe")).Send(req).EndStruct(&probe)
		if len(errs) == 0 {
			if probe.UpdateAvailable {
				log.Info("Update available")
			} else {
				log.Info("Update not available")
				os.Exit(0)
			}
		} else {
			log.Fatal("Invalid response from UpdateHub Agent")
		}
	}

	d.Run()
}

func buildURL(path string) string {
	return fmt.Sprintf("http://localhost:8080/%s", path[1:])
}
