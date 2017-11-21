/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package installifdifferent

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"

	"github.com/spf13/afero"
)

type PatternType int

const (
	CustomPattern      PatternType = iota
	UBootPattern       PatternType = iota
	LinuxKernelPattern PatternType = iota
)

type Pattern struct {
	Type       PatternType
	RegExp     string `json:"regexp"`
	Seek       int64  `json:"seek"`
	BufferSize int64  `json:"buffer-size"`

	FileSystemBackend afero.Fs
}

func (p *Pattern) IsValid() bool {
	_, err := regexp.Compile(p.RegExp)

	if err == nil && p.Seek >= 0 && p.BufferSize >= 0 && int(p.Type) >= int(CustomPattern) && int(p.Type) <= int(LinuxKernelPattern) {
		return true
	}

	return false
}

func (p *Pattern) Capture(target io.ReadSeeker) (string, error) {
	switch p.Type {
	case LinuxKernelPattern:
		kfi := NewKernelFileInfo(p.FileSystemBackend, target)
		return kfi.Version, nil
	case UBootPattern:
		return CaptureTextFromBinaryFile(target, p.RegExp), nil
	case CustomPattern:
		data := make([]byte, p.BufferSize)

		target.Seek(p.Seek, io.SeekStart)
		target.Read(data)

		re, _ := regexp.Compile(p.RegExp)
		matched := re.FindStringSubmatch(string(data))
		if matched != nil && len(matched) > 0 {
			return matched[0], nil
		}

		return "", nil
	}

	return "", fmt.Errorf("unknown pattern type")
}

func NewPatternFromInstallIfDifferentObject(fsb afero.Fs, pattern map[string]interface{}) (*Pattern, error) {
	p := &Pattern{FileSystemBackend: fsb}

	s, ok := pattern["pattern"].(string)
	if ok {
		if s == "u-boot" {
			p.Type = UBootPattern
			p.RegExp = `U-Boot(?: SPL)? (\S+) \(.*\)`
			p.Seek = 0
			p.BufferSize = 0
			return p, nil
		}

		if s == "linux-kernel" {
			p.Type = LinuxKernelPattern
			p.RegExp = ``
			p.Seek = 0
			p.BufferSize = 0
			return p, nil
		}
	}

	patternMap, ok := pattern["pattern"].(map[string]interface{})
	if ok {
		p.Type = CustomPattern

		// we can safely ignore the error here because this comes from
		// a successful unmarshalled json. worst case scenario, the
		// unmarshal below will catch any error
		b, _ := json.Marshal(patternMap)

		err := json.Unmarshal(b, p)
		if err != nil {
			return nil, err
		}

		return p, nil
	}

	return nil, fmt.Errorf("install-if-different pattern is unknown")
}
