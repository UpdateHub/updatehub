/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package server

import (
	"path"

	"github.com/OSSystems/pkg/log"
	"github.com/UpdateHub/updatehub/metadata"
	"github.com/fsnotify/fsnotify"
)

type Daemon struct {
	fswatcher       *fsnotify.Watcher
	backend         *ServerBackend
	done            chan bool
	started         chan bool
	metadataWritten chan bool
}

func NewDaemon(sb *ServerBackend) (*Daemon, error) {
	d := &Daemon{
		backend:         sb,
		done:            make(chan bool),
		started:         make(chan bool),
		metadataWritten: make(chan bool),
	}

	fswatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err = fswatcher.Add(sb.path); err != nil {
		return nil, err
	}

	d.fswatcher = fswatcher

	return d, nil
}

func (d *Daemon) Run() {
	go func() {
		for {
			select {
			case event := <-d.fswatcher.Events:
				switch event.Op {
				case fsnotify.Remove:
					if event.Name == d.backend.path {
						d.done <- true
					}
				case fsnotify.Write:
					umFileName := path.Join(d.backend.path, metadata.UpdateMetadataFilename)
					if event.Name == umFileName {
						err := d.backend.ProcessDirectory()
						if err != nil {
							log.Error(err)
						}
						d.metadataWritten <- true
					}
				}
			case err := <-d.fswatcher.Errors:
				log.Error(err)
			}
		}
	}()

	d.started <- true
	<-d.done
}
