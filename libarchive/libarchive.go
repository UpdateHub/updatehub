/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package libarchive

// FIXME: test this whole file

/*
#cgo pkg-config: libarchive
#include <archive.h>
#include <archive_entry.h>
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"io"
	"os"
	"reflect"
	"unsafe"

	"github.com/OSSystems/pkg/log"
)

// Archive is a wrapper for "C.struct_archive"
type Archive struct {
	archive *C.struct_archive
}

// ArchiveEntry is a wrapper for "C.struct_archive_entry"
type ArchiveEntry struct {
	entry *C.struct_archive_entry
}

// API is the libarchive API provided by this package
type API interface {
	NewRead() Archive
	ReadSupportFilterAll(a Archive) error
	ReadSupportFormatRaw(a Archive) error
	ReadSupportFormatAll(a Archive) error
	ReadSupportFormatEmpty(a Archive) error
	ReadOpenFileName(a Archive, filename string, blockSize int) error
	ReadFree(a Archive)
	ReadNextHeader(a Archive, e *ArchiveEntry) error
	ReadData(a Archive, buffer []byte, length int) (int, error)
	ReadDataSkip(a Archive) error
	WriteDiskNew() Archive
	WriteDiskSetOptions(a Archive, flags int)
	WriteDiskSetStandardLookup(a Archive)
	WriteFree(a Archive)
	WriteHeader(a Archive, e ArchiveEntry) error
	WriteFinishEntry(a Archive) error
	EntrySize(e ArchiveEntry) int64
	EntrySizeIsSet(e ArchiveEntry) bool
	EntryPathname(e ArchiveEntry) string
	Unpack(tarballPath string, targetPath string, enableRaw bool) error
}

// LibArchive is the default implementation of API
type LibArchive struct {
}

// NewRead is a wrapper for "C.archive_read_new()"
func (la LibArchive) NewRead() Archive {
	a := Archive{}
	a.archive = C.archive_read_new()
	return a
}

// ReadSupportFilterAll is a wrapper for "C.archive_read_support_filter_all()"
func (la LibArchive) ReadSupportFilterAll(a Archive) error {
	r := C.archive_read_support_filter_all(a.archive)

	if r != C.ARCHIVE_OK {
		return fmt.Errorf(C.GoString(C.archive_error_string(a.archive)))
	}

	return nil
}

// ReadSupportFormatRaw is a wrapper for "C.archive_read_support_format_raw()"
func (la LibArchive) ReadSupportFormatRaw(a Archive) error {
	r := C.archive_read_support_format_raw(a.archive)

	if r != C.ARCHIVE_OK {
		return fmt.Errorf(C.GoString(C.archive_error_string(a.archive)))
	}

	return nil
}

// ReadSupportFormatAll is a wrapper for "C.archive_read_support_format_all()"
func (la LibArchive) ReadSupportFormatAll(a Archive) error {
	r := C.archive_read_support_format_all(a.archive)

	if r != C.ARCHIVE_OK {
		return fmt.Errorf(C.GoString(C.archive_error_string(a.archive)))
	}

	return nil
}

// ReadSupportFormatEmpty is a wrapper for "C.archive_read_support_format_empty()"
func (la LibArchive) ReadSupportFormatEmpty(a Archive) error {
	r := C.archive_read_support_format_empty(a.archive)

	if r != C.ARCHIVE_OK {
		return fmt.Errorf(C.GoString(C.archive_error_string(a.archive)))
	}

	return nil
}

// ReadOpenFileName is a wrapper for "C.archive_read_open_filename()"
func (la LibArchive) ReadOpenFileName(a Archive, filename string, blockSize int) error {
	cFilename := C.CString(filename)
	r := C.archive_read_open_filename(a.archive, cFilename, C.size_t(blockSize))
	C.free(unsafe.Pointer(cFilename))

	if r != C.ARCHIVE_OK {
		return fmt.Errorf(C.GoString(C.archive_error_string(a.archive)))
	}

	return nil
}

// ReadFree is a wrapper for "C.archive_read_free()"
func (la LibArchive) ReadFree(a Archive) {
	C.archive_read_free(a.archive)
}

// ReadNextHeader is a wrapper for "C.archive_read_next_header()"
func (la LibArchive) ReadNextHeader(a Archive, e *ArchiveEntry) error {
	r := C.archive_read_next_header(a.archive, &e.entry)

	if r == C.ARCHIVE_EOF {
		return io.EOF
	}

	if r != C.ARCHIVE_OK {
		return fmt.Errorf(C.GoString(C.archive_error_string(a.archive)))
	}

	return nil
}

// ReadData is a wrapper for "C.archive_read_data()"
func (la LibArchive) ReadData(a Archive, buffer []byte, length int) (int, error) {
	r := C.archive_read_data(a.archive, unsafe.Pointer(&buffer[0]), C.size_t(length))

	if r < 0 {
		return int(r), fmt.Errorf(C.GoString(C.archive_error_string(a.archive)))
	}

	return int(r), nil
}

// ReadDataSkip is a wrapper for "C.archive_read_data_skip()"
func (la LibArchive) ReadDataSkip(a Archive) error {
	r := C.archive_read_data_skip(a.archive)

	if r == C.ARCHIVE_EOF {
		return io.EOF
	}

	if r != C.ARCHIVE_OK {
		return fmt.Errorf(C.GoString(C.archive_error_string(a.archive)))
	}

	return nil
}

// WriteDiskNew is a wrapper for "C.archive_write_disk_new()"
func (la LibArchive) WriteDiskNew() Archive {
	a := Archive{}
	a.archive = C.archive_write_disk_new()
	return a
}

// WriteDiskSetOptions is a wrapper for "C.archive_write_disk_set_options()"
func (la LibArchive) WriteDiskSetOptions(a Archive, flags int) {
	C.archive_write_disk_set_options(a.archive, C.int(flags))
}

// WriteDiskSetStandardLookup is a wrapper for "C.archive_write_disk_set_standard_lookup()"
func (la LibArchive) WriteDiskSetStandardLookup(a Archive) {
	C.archive_write_disk_set_standard_lookup(a.archive)
}

// WriteFree is a wrapper for "C.archive_write_free()"
func (la LibArchive) WriteFree(a Archive) {
	C.archive_write_free(a.archive)
}

// WriteHeader is a wrapper for "C.archive_write_header()"
func (la LibArchive) WriteHeader(a Archive, e ArchiveEntry) error {
	r := C.archive_write_header(a.archive, e.entry)

	if r != C.ARCHIVE_OK {
		return fmt.Errorf(C.GoString(C.archive_error_string(a.archive)))
	}

	return nil
}

// WriteFinishEntry is a wrapper for "C.archive_write_finish_entry()"
func (la LibArchive) WriteFinishEntry(a Archive) error {
	r := C.archive_write_finish_entry(a.archive)

	if r != C.ARCHIVE_OK {
		return fmt.Errorf(C.GoString(C.archive_error_string(a.archive)))
	}

	return nil
}

// EntrySize is a wrapper for "C.archive_entry_size()"
func (la LibArchive) EntrySize(e ArchiveEntry) int64 {
	r := C.archive_entry_size(e.entry)
	return int64(r)
}

// EntrySizeIsSet is a wrapper for "C.archive_entry_size_is_set()"
func (la LibArchive) EntrySizeIsSet(e ArchiveEntry) bool {
	r := C.archive_entry_size_is_set(e.entry)

	if r == 0 {
		return false
	}

	return true
}

// EntryPathname is a wrapper for ""
func (la LibArchive) EntryPathname(e ArchiveEntry) string {
	return C.GoString(C.archive_entry_pathname(e.entry))
}

// Unpack contains the algorithm to extract files from a tarball and
// put them on a directory
func (la LibArchive) Unpack(tarballPath string, targetPath string, enableRaw bool) error {
	originalDir, err := os.Getwd()
	if err != nil {
		return err
	}

	err = os.Chdir(targetPath)
	if err != nil {
		return err
	}
	defer os.Chdir(originalDir)

	err = extractTarball(la, tarballPath, enableRaw)
	if err != nil {
		return err
	}

	return nil
}

// Reader is an abstraction that implements the io.Reader interface
type Reader struct {
	API                 // the implementation being used
	Archive     Archive // the Archive being used
	ChunkSize   int     // the chunk size being used
	ArchivePath string  // the path of the Archive being used
}

// NewReader is a factory method used to create a new Reader. Must
// receive an API implementation, the filePath on which the file will
// be read and the chunkSize used for reading
func NewReader(api API, filePath string, chunkSize int) (*Reader, error) {
	a := api.NewRead()

	err := api.ReadSupportFilterAll(a)
	if err != nil {
		api.ReadFree(a)
		return nil, err
	}

	err = api.ReadSupportFormatRaw(a)
	if err != nil {
		api.ReadFree(a)
		return nil, err
	}

	err = api.ReadSupportFormatEmpty(a)
	if err != nil {
		api.ReadFree(a)
		return nil, err
	}

	err = api.ReadSupportFormatAll(a)
	if err != nil {
		api.ReadFree(a)
		return nil, err
	}

	err = api.ReadOpenFileName(a, filePath, chunkSize)
	if err != nil {
		api.ReadFree(a)
		return nil, err
	}

	r := &Reader{api, a, chunkSize, filePath}

	return r, nil
}

// Read implements the io.Reader interface
func (r Reader) Read(p []byte) (n int, err error) {
	n, err = r.API.ReadData(r.Archive, p, r.ChunkSize)

	if n < 0 && err != nil {
		return 0, err
	}

	if n == 0 && err == nil {
		return 0, io.EOF
	}

	return n, err
}

// ReadNextHeader setups the Archive for a set of reads
func (r Reader) ReadNextHeader() error {
	e := ArchiveEntry{}
	return r.API.ReadNextHeader(r.Archive, &e)
}

// Free frees the Archive
func (r Reader) Free() {
	r.API.ReadFree(r.Archive)
}

// ExtractFile extracts a single file from the associated Archive to
// the 'target' interface
func (r Reader) ExtractFile(filename string, target io.Writer) error {
	for {
		e := ArchiveEntry{}
		err := r.API.ReadNextHeader(r.Archive, &e)

		if err != nil {
			break
		}

		p := r.API.EntryPathname(e)

		if p == filename {
			var buff *C.void
			cBuffer := unsafe.Pointer(buff)
			var size C.size_t
			var offset C.__LA_INT64_T

			for {
				r := C.archive_read_data_block(r.Archive.archive, &cBuffer, &size, &offset)
				if r == C.ARCHIVE_EOF {
					break
				}

				slice := &reflect.SliceHeader{Data: uintptr(cBuffer), Len: int(size), Cap: int(size)}
				goBuffer := *(*[]byte)(unsafe.Pointer(slice))

				_, err = target.Write(goBuffer)
				if err != nil {
					return err
				}
			}

			return nil
		}

		r.API.ReadDataSkip(r.Archive)
		if err != nil {
			break
		}
	}

	finalErr := fmt.Errorf("file '%s' not found in: '%s'", filename, r.ArchivePath)
	log.Error(finalErr)
	return finalErr
}

func extractTarball(api API, filename string, enableRaw bool) error {
	source := api.NewRead()
	defer api.ReadFree(source)

	err := api.ReadSupportFilterAll(source)
	if err != nil {
		return err
	}

	err = api.ReadSupportFormatAll(source)
	if err != nil {
		return err
	}

	if enableRaw {
		err = api.ReadSupportFormatRaw(source)
		if err != nil {
			return err
		}
	}

	flags := C.ARCHIVE_EXTRACT_TIME
	flags |= C.ARCHIVE_EXTRACT_PERM
	flags |= C.ARCHIVE_EXTRACT_ACL
	flags |= C.ARCHIVE_EXTRACT_FFLAGS
	flags |= C.ARCHIVE_EXTRACT_OWNER
	flags |= C.ARCHIVE_EXTRACT_FFLAGS
	flags |= C.ARCHIVE_EXTRACT_XATTR

	target := api.WriteDiskNew()
	defer api.WriteFree(target)

	api.WriteDiskSetOptions(target, flags)
	api.WriteDiskSetStandardLookup(target)

	err = api.ReadOpenFileName(source, filename, 10240)
	if err != nil {
		return err
	}

	entry := ArchiveEntry{}
	for {
		err = api.ReadNextHeader(source, &entry)
		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		err = api.WriteHeader(target, entry)
		if err != nil {
			return err
		}

		if !api.EntrySizeIsSet(entry) || api.EntrySize(entry) > 0 {
			err = copyData(api, source, target)
			if err != nil {
				return err
			}

		}

		err = api.WriteFinishEntry(target)
		if err != nil {
			return err
		}
	}

	return nil
}

func copyData(api API, ar Archive, aw Archive) error {
	var buff *C.void
	b := unsafe.Pointer(buff)
	var size C.size_t
	var offset C.__LA_INT64_T

	for {
		r := C.archive_read_data_block(ar.archive, &b, &size, &offset)

		if r == C.ARCHIVE_EOF {
			return nil
		}

		if r < C.ARCHIVE_OK {
			return fmt.Errorf(C.GoString(C.archive_error_string(ar.archive)))
		}

		w := C.archive_write_data_block(aw.archive, b, size, offset)

		if w < C.ARCHIVE_OK {
			return fmt.Errorf(C.GoString(C.archive_error_string(aw.archive)))
		}
	}
}
