/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package copy

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/OSSystems/pkg/log"
	shellwords "github.com/mattn/go-shellwords"
	"github.com/spf13/afero"
	"github.com/updatehub/updatehub/libarchive"
	"github.com/updatehub/updatehub/utils"
)

type Interface interface {
	Copy(wr io.Writer, rd io.Reader, timeout time.Duration, cancel <-chan bool, chunkSize int, skip int, count int, compressed bool) (bool, error)
	CopyFile(
		fsBackend afero.Fs,
		libarchiveBackend libarchive.API,
		sourcePath string,
		targetPath string,
		chunkSize int,
		skip int,
		seek int,
		count int,
		truncate bool,
		compressed bool) error
	CopyToProcessStdin(
		fsBackend afero.Fs,
		libarchiveBackend libarchive.API,
		sourcePath string,
		processCmdline string,
		compressed bool) error
}

type ExtendedIO struct {
}

// Copy copies from rd to wr until EOF or timeout is reached on rd or it was cancelled
func (eio ExtendedIO) Copy(wr io.Writer, rd io.Reader, timeout time.Duration, cancel <-chan bool, chunkSize int, skip int, count int, compressed bool) (bool, error) {
	if chunkSize < 1 {
		finalErr := fmt.Errorf("copy error: chunkSize can't be less than 1")
		log.Error(finalErr)
		return false, finalErr
	}

	len := make(chan int)
	buf := make([]byte, chunkSize)
	readErrChan := make(chan error)
	toSkip := skip

Loop:
	for i := 0; i != count; i++ {
		go func() {
			n, err := rd.Read(buf)
			if n == 0 && err != nil {
				if err != io.EOF {
					readErrChan <- err
				}
				close(len)
			} else {
				len <- n
			}
		}()

		select {
		case err, ok := <-readErrChan:
			if ok {
				close(readErrChan)
			}
			
			return false, err
		case <-cancel:
			return true, nil
		case <-time.After(timeout):
			finalErr := fmt.Errorf("copy error: timeout")
			log.Error(finalErr)
			return false, finalErr
		case n, ok := <-len:
			if !ok {
				break Loop
			}

			// skip is done like this in compressed files
			if compressed && toSkip > 0 {
				toSkip--
				i--
			} else {
				_, err := wr.Write(buf[0:n])
				if err != nil {
					finalErr := fmt.Errorf("copy error: write: %s", err)
					log.Error(finalErr)
					return false, finalErr
				}
			}
		}
	}

	return false, nil
}

func (eio ExtendedIO) CopyFile(
	fsBackend afero.Fs,
	libarchiveBackend libarchive.API,
	sourcePath string,
	targetPath string,
	chunkSize int,
	skip int,
	seek int,
	count int,
	truncate bool,
	compressed bool) error {

	var err error

	flags := os.O_RDWR | os.O_CREATE
	if truncate {
		flags = flags | os.O_TRUNC
	}

	target, err := fsBackend.OpenFile(targetPath, flags, 0666)
	if err != nil {
		log.Error(err)
		return err
	}
	defer target.Close()

	_, err = target.Seek(int64(seek*chunkSize), io.SeekStart)
	if err != nil {
		log.Error(err)
		return err
	}

	err = eio.sharedCopyLogic(fsBackend, libarchiveBackend, target, sourcePath, chunkSize, skip, count, compressed)

	return err
}

func (eio ExtendedIO) CopyToProcessStdin(
	fsBackend afero.Fs,
	libarchiveBackend libarchive.API,
	sourcePath string,
	processCmdline string,
	compressed bool) error {

	// processCmdline
	p := shellwords.NewParser()
	list, err := p.Parse(processCmdline)
	if err != nil {
		finalErr := fmt.Errorf("copy to process stdin error: failed to parse cmdline: %s", err)
		log.Error(finalErr)
		return finalErr
	}

	cmd := exec.Command(list[0], list[1:]...)
	processStdin, err := cmd.StdinPipe()
	if err != nil {
		finalErr := fmt.Errorf("copy to process stdin error: failed to get stdin pipe: %s", err)
		log.Error(finalErr)
		return finalErr
	}

	err = cmd.Start()
	if err != nil {
		finalErr := fmt.Errorf("copy to process stdin error: failed to start process: %s", err)
		log.Error(finalErr)
		return finalErr
	}

	err = eio.sharedCopyLogic(fsBackend, libarchiveBackend, processStdin, sourcePath,
		utils.ChunkSize, 0, -1, compressed)

	processStdin.Close()

	if err != nil {
		return err
	}

	err = cmd.Wait()
	if waitErr, ok := err.(*exec.ExitError); ok {
		if !waitErr.Success() {
			return waitErr
		}
	}

	return err
}

func (eio ExtendedIO) sharedCopyLogic(
	fsBackend afero.Fs,
	libarchiveBackend libarchive.API,
	target io.Writer,
	sourcePath string,
	chunkSize int,
	skip int,
	count int,
	compressed bool) error {

	var source io.Reader

	if compressed {
		reader, readerErr := libarchive.NewReader(libarchiveBackend, sourcePath, chunkSize)
		if readerErr != nil {
			finalErr := fmt.Errorf("failed to create libarchive reader: %s", readerErr)
			log.Error(finalErr)
			return finalErr
		}
		defer reader.Free()

		nextHeaderErr := reader.ReadNextHeader()

		// empty file special case
		if nextHeaderErr == io.EOF {
			_, writeErr := target.Write([]byte(""))
			if writeErr != nil {
				finalErr := fmt.Errorf("failed to write empty file: %s", writeErr)
				log.Error(finalErr)
				return finalErr
			}

			return nil
		}

		if nextHeaderErr != nil {
			finalErr := fmt.Errorf("failed to get next archive header: %s", nextHeaderErr)
			log.Error(finalErr)
			return finalErr
		}

		// for compressed files the "skip" is done inside the "Copy"
		// function

		source = reader
	} else {
		file, fileErr := fsBackend.Open(sourcePath)
		if fileErr != nil {
			if pathErr, ok := fileErr.(*os.PathError); ok {
				log.Error(pathErr)
				return pathErr
			}
			log.Error(fileErr)
			return fileErr
		}
		defer file.Close()

		_, seekErr := file.Seek(int64(skip*chunkSize), io.SeekStart)
		if seekErr != nil {
			log.Error(seekErr)
			return seekErr
		}

		source = file
	}

	// copy
	cancel := make(chan bool)
	_, err := eio.Copy(target, source, time.Hour, cancel, chunkSize, skip, count, compressed)

	return err
}
