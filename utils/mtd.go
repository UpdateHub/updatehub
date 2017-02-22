package utils

/*
#include <stdlib.h>
#include <string.h>
#include <fcntl.h>
#include <unistd.h>
#include <errno.h>
#include <sys/ioctl.h>
#include <mtd/mtd-abi.h>

int open_wrapper(const char *pathname, int flags) {
    return open(pathname, flags);
}

int errno_wrapper() {
    return errno;
}

int ioctl_wrapper(int fd, unsigned long request, struct mtd_info_user *mtd) {
    return ioctl(fd, request, mtd);
}
*/
import "C"

import (
	"bufio"
	"fmt"
	"regexp"
	"unsafe"

	"github.com/spf13/afero"
)

type MtdUtils interface {
	MtdIsNAND(devicepath string) (bool, error)
	GetTargetDeviceFromMtdName(fsBackend afero.Fs, mtdname string) (string, error)
}

type MtdUtilsImpl struct {
}

func (m MtdUtilsImpl) MtdIsNAND(devicepath string) (bool, error) {
	// FIXME: test this method (maybe we should rewrite this method in
	// C and put it on a separated library, and then test it
	// separatedly. check if cgo has support for it)
	cDevicepath := C.CString(devicepath)
	mtdFD := C.open_wrapper(cDevicepath, C.O_RDWR)
	defer C.free(unsafe.Pointer(cDevicepath))

	if mtdFD == -1 {
		return false, fmt.Errorf("Couldn't open flash device '%s': %s", devicepath, C.GoString(C.strerror(C.errno_wrapper())))
	}
	defer C.close(mtdFD)

	mtd := C.struct_mtd_info_user{}

	if C.ioctl_wrapper(mtdFD, C.MEMGETINFO, &mtd) < 0 {
		return false, fmt.Errorf("Error executing MEMGETINFO ioctl on '%s': %s", devicepath, C.GoString(C.strerror(C.errno_wrapper())))
	}

	if C.mtd_type_is_nand_user(&mtd) != 0 {
		return true, nil
	}

	return false, nil
}

func (m MtdUtilsImpl) GetTargetDeviceFromMtdName(fsBackend afero.Fs, mtdname string) (string, error) {
	procMtd, err := fsBackend.Open("/proc/mtd")
	if err != nil {
		return "", err
	}
	defer procMtd.Close()

	scanner := bufio.NewScanner(procMtd)
	firstLineSkipped := false // file header
	for scanner.Scan() {
		// process ""
		if !firstLineSkipped {
			firstLineSkipped = true
			continue
		}

		r := regexp.MustCompile(`^(mtd\d): (\d+) (\d+) "(.*)"$`)
		matched := r.FindStringSubmatch(scanner.Text())

		if matched != nil && len(matched) == 5 && matched[4] == mtdname {
			mtddevice := matched[1]
			return fmt.Sprintf("/dev/%s", mtddevice), nil
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("Couldn't find a flash device corresponding to the mtdname '%s'", mtdname)
}
