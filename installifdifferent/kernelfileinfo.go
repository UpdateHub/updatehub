/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package installifdifferent

import (
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"path"
	"regexp"
	"unicode"

	"github.com/spf13/afero"
	"github.com/updatehub/updatehub/libarchive"
	"github.com/updatehub/updatehub/utils"
)

type LinuxArch int

const (
	UnknownLinuxArch LinuxArch = iota
	ARMLinuxArch     LinuxArch = iota
	x86LinuxArch     LinuxArch = iota
)

type KernelType int

const (
	UnknownKernelType KernelType = iota
	zImageKernelType  KernelType = iota
	uImageKernelType  KernelType = iota
	bzImageKernelType KernelType = iota
)

type KernelFileInfo struct {
	Arch    LinuxArch
	Type    KernelType
	Version string

	FileSystemBackend afero.Fs
}

func NewKernelFileInfo(fsb afero.Fs, file io.ReadSeeker) *KernelFileInfo {
	kfi := &KernelFileInfo{FileSystemBackend: fsb}

	if isARMzImage(file) {
		kfi.Arch = ARMLinuxArch
		kfi.Type = zImageKernelType
	} else if isARMuImage(file) {
		kfi.Arch = ARMLinuxArch
		kfi.Type = uImageKernelType
	} else if isx86bzImage(file) {
		kfi.Arch = x86LinuxArch
		kfi.Type = bzImageKernelType
	} else if isx86zImage(file) {
		kfi.Arch = x86LinuxArch
		kfi.Type = zImageKernelType
	}

	// since this uses only "TempDir()" we don't need to assign a
	// value for the CmdlineExecuter
	fsh := &utils.FileSystem{}

	version := kfi.captureVersion(fsh, file)
	re, _ := regexp.Compile(`(\d+.?\.[^\s]+)`)
	matched := re.FindStringSubmatch(version)
	if matched != nil && len(matched) == 2 {
		kfi.Version = matched[1]
	}

	return kfi
}

// we can ignore errors here since if it fails, just means it is
// not the image type being tested
func isARMzImage(file io.ReadSeeker) bool {
	file.Seek(36, io.SeekStart)
	var magic int32 // 4 bytes
	binary.Read(file, binary.LittleEndian, &magic)

	if magic == 0x016f2818 {
		return true
	}

	return false
}

// we can ignore errors here since if it fails, just means it is
// not the image type being tested
func isARMuImage(file io.ReadSeeker) bool {
	file.Seek(0, io.SeekStart)
	var magic int32 // 4 bytes
	binary.Read(file, binary.BigEndian, &magic)

	if magic == 0x27051956 {
		return true
	}

	return false
}

// we can ignore errors here since if it fails, just means it is
// not the image type being tested
func isx86bzImage(file io.ReadSeeker) bool {
	file.Seek(510, io.SeekStart)
	var magic uint16 // 2 bytes
	binary.Read(file, binary.LittleEndian, &magic)

	file.Seek(529, io.SeekStart)

	var bzip byte
	binary.Read(file, binary.LittleEndian, &bzip)

	if magic == 0xaa55 && bzip == 1 {
		return true
	}

	return false
}

// we can ignore errors here since if it fails, just means it is
// not the image type being tested
func isx86zImage(file io.ReadSeeker) bool {
	file.Seek(510, io.SeekStart)
	var magic uint16 // 2 bytes
	binary.Read(file, binary.LittleEndian, &magic)

	file.Seek(529, io.SeekStart)

	var gzip byte
	binary.Read(file, binary.LittleEndian, &gzip)

	if magic == 0xaa55 && gzip == 0 {
		return true
	}

	return false
}

// we can ignore errors here since if it fails, just means it is
// not the image type being tested
func (kfi *KernelFileInfo) captureVersion(fsh utils.FileSystemHelper, file io.ReadSeeker) string {
	if kfi.Arch == ARMLinuxArch && kfi.Type == uImageKernelType {
		file.Seek(32, io.SeekStart)
		version := make([]byte, 32)
		file.Read(version)
		n := bytes.IndexByte(version, 0)
		return string(version[:n])
	} else if kfi.Arch == ARMLinuxArch && kfi.Type == zImageKernelType {
		file.Seek(0, io.SeekStart)

		buffer := new(bytes.Buffer)
		buffer.ReadFrom(file)

		// "0x1f 0x8b 0x08" is the beginning of the gzipped kernel file
		skip := bytes.Index(buffer.Bytes(), []byte{0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00})

		content := buffer.Bytes()[skip:] // gzipped kernel file

		// Creates a gzipped file of all remaining data from file
		gzippedFile, _ := afero.TempFile(kfi.FileSystemBackend, "", "gzippedFile")
		defer os.Remove(gzippedFile.Name())
		gzippedFile.Write(content)
		gzippedFile.Close()

		tempdir, _ := fsh.TempDir(kfi.FileSystemBackend, "kernelfileinfo")
		defer os.RemoveAll(tempdir)

		// Decompress the gzipped kernel file
		la := &libarchive.LibArchive{}
		la.Unpack(gzippedFile.Name(), tempdir, true)

		data, _ := kfi.FileSystemBackend.OpenFile(path.Join(tempdir, "data"), os.O_RDONLY, 0)
		defer data.Close()

		return CaptureTextFromBinaryFile(data, `Linux version (\S+).*`)
	} else if kfi.Arch == x86LinuxArch {
		file.Seek(0x20E, io.SeekStart)
		var offset uint16 // 2 bytes
		binary.Read(file, binary.LittleEndian, &offset)
		file.Seek(int64(offset)+0x200, io.SeekStart)
		version := make([]byte, 512)
		file.Read(version)
		n := bytes.IndexByte(version, 0)
		return string(version[:n])
	}

	return ""
}

func CaptureTextFromBinaryFile(file io.ReadSeeker, regularExpression string) string {
	trailing := make([]byte, 4)
	bytesRead := make([]byte, 1)
	index := 0
	var buffer bytes.Buffer

	for {
		n, err := file.Read(bytesRead)

		if err == io.EOF && n == 0 {
			// end of file
			break
		}

		if err != nil {
			return ""
		}

		if unicode.IsPrint(rune(bytesRead[0])) {
			if index == 4 {
				buffer.WriteString(string(bytesRead[0]))
			} else {
				trailing[index] = bytesRead[0]
				index++
				if index == 4 {
					buffer.WriteString(string(trailing))
				}
			}
		} else {
			if index == 4 {
				re, _ := regexp.Compile(regularExpression)
				matched := re.FindStringSubmatch(buffer.String())
				if matched != nil && len(matched) == 2 {
					return matched[1]
				}

				buffer.Reset()
			}

			index = 0
		}
	}

	return ""
}
