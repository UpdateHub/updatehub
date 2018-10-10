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
	"syscall"
	"time"

	"github.com/OSSystems/pkg/log"
	linuxproc "github.com/c9s/goprocinfo/linux"
	"github.com/parnurzeal/gorequest"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	_ "github.com/UpdateHub/updatehub/installmodes/copy"
	_ "github.com/UpdateHub/updatehub/installmodes/flash"
	_ "github.com/UpdateHub/updatehub/installmodes/imxkobs"
	_ "github.com/UpdateHub/updatehub/installmodes/raw"
	_ "github.com/UpdateHub/updatehub/installmodes/tarball"
	_ "github.com/UpdateHub/updatehub/installmodes/ubifs"
	"github.com/UpdateHub/updatehub/libarchive"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/UpdateHub/updatehub/server"
	"github.com/UpdateHub/updatehub/updatehub"
)

type AgentInfo struct {
	Version   string                    `json:"version"`
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
	probe := cmd.PersistentFlags().Bool("probe", false, "probe the updatehub for update")
	mount := cmd.PersistentFlags().StringP("mount", "m", "", "device to mount")
	fstype := cmd.PersistentFlags().StringP("fstype", "f", "", "filesystem type of device to mount")

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

	if *mount != "" {
		mounts, err := linuxproc.ReadMounts("/proc/mounts")
		if err != nil {
			log.Fatal(err)
			os.Exit(1)
		}

		for _, mount := range mounts.Mounts {
			if mount.MountPoint == path {
				log.Fatalf("%s: already mounted", path)
				os.Exit(1)
			}
		}

		if err = syscall.Mount(*mount, path, *fstype, syscall.MS_RDONLY, ""); err != nil {
			log.Fatal(err)
			os.Exit(1)
		}
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
		if err := http.ListenAndServe(":8088", router.HTTPRouter); err != nil {
			log.Fatal(err)
		}
	}()

	var info *AgentInfo

	// Wait and check for UpdateHub Agent is running
	for *probe {
		_, _, errs := gorequest.New().Get(buildURL("/info")).EndStruct(&info)
		if len(errs) == 0 {
			log.Info("UpdateHub Agent is running")

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

			break
		}

		log.Info("Waiting for updatehub")
		time.Sleep(time.Second)
	}

	d.Run()
}

func buildURL(path string) string {
	return fmt.Sprintf("http://localhost:8080/%s", path[1:])
}
