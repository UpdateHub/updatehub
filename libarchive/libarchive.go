package libarchive

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
	"unsafe"
)

func Copy(target io.Writer, sourcePath string, chunkSize int, skip int, seek int, count int, truncate bool) error {
	var r C.int

	a := C.archive_read_new()
	C.archive_read_support_filter_all(a)
	C.archive_read_support_format_raw(a)
	C.archive_read_support_format_empty(a)

	r = C.archive_read_open_filename(a, C.CString(sourcePath), C.size_t(chunkSize))
	defer C.archive_read_free(a)

	if r != C.ARCHIVE_OK {
		return fmt.Errorf(C.GoString(C.archive_error_string(a)))
	}

	var entry *C.struct_archive_entry
	r = C.archive_read_next_header(a, &entry)

	// empty file special case
	if r == C.ARCHIVE_EOF {
		_, err := target.Write([]byte(""))
		if err != nil {
			return err
		}

		return nil
	}

	if r != C.ARCHIVE_OK {
		return fmt.Errorf("Error reading header from '%s': %s", sourcePath, C.GoString(C.archive_error_string(a)))
	}

	toSkip := skip
	looped := 0
	for looped != count {
		data := make([]byte, chunkSize)
		bytesRead := C.archive_read_data(a, unsafe.Pointer(&data[0]), C.size_t(chunkSize))

		if bytesRead < 0 {
			return fmt.Errorf("Error reading data from '%s': %s", sourcePath, C.GoString(C.archive_error_string(a)))
		}

		if bytesRead == 0 {
			break
		}

		if toSkip > 0 {
			toSkip--
		} else {
			dataToBeWritten := make([]byte, bytesRead)
			copy(dataToBeWritten, data)
			_, err := target.Write(dataToBeWritten)
			if err != nil {
				return err
			}

			looped++
		}
	}

	return nil
}
