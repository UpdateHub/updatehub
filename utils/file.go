/*
 * UpdateHub
 * Copyright (C) 2017
 * O.S. Systems Sofware LTDA: contato@ossystems.com.br
 *
 * SPDX-License-Identifier:     GPL-2.0
 */

package utils

import (
	"fmt"
	"os"
	"os/user"
	"strconv"

	"github.com/spf13/afero"
)

type Permissions interface {
	ApplyChmod(fsb afero.Fs, filepath string, modestr string) error
	ApplyChown(filepath string, uid interface{}, gid interface{}) error
}

type PermissionsDefaultImpl struct {
}

func (pdi *PermissionsDefaultImpl) ApplyChmod(fsb afero.Fs, filepath string, modestr string) error {
	if modestr == "" {
		return nil
	}

	mode, err := strconv.ParseUint(modestr, 8, 32)
	if err != nil {
		return err
	}

	fi, err := os.Lstat(filepath)
	if err != nil {
		return err
	}

	if fi.Mode()&os.ModeSymlink != os.ModeSymlink {
		err := fsb.Chmod(filepath, (os.FileMode)(mode))
		if err != nil {
			return err
		}
	}

	return nil
}

// FIXME: this cannot be tested yet with unit tests because requires
// mock for: os.Chown, os.Lchown, os.Lstat, user.Lookup and
// user.LookupGroup. Since we can't change ownership on a real
// filesystem, this must be tested through integration.
func (pdi *PermissionsDefaultImpl) ApplyChown(filepath string, uid interface{}, gid interface{}) error {
	if uid == nil && gid == nil {
		return nil
	}

	// uid parsing
	uidint := 0
	if uid != nil {
		var i int64

		uidstr, ok := uid.(string)
		if !ok {
			i, ok = uid.(int64)
			if !ok {
				return fmt.Errorf("uid must be string or int")
			}
		} else {
			u, err := user.Lookup(uidstr)
			if err != nil {
				return err
			}

			i, err = strconv.ParseInt(u.Uid, 10, 0)
			if err != nil {
				return err
			}
		}

		uidint = int(i)
	}

	// gid parsing
	gidint := 0
	if gid != nil {
		var i int64

		gidstr, ok := gid.(string)
		if !ok {
			i, ok = gid.(int64)
			if !ok {
				return fmt.Errorf("gid must be string or int")
			}
		} else {
			g, err := user.LookupGroup(gidstr)
			if err != nil {
				return err
			}

			i, err = strconv.ParseInt(g.Gid, 10, 0)
			if err != nil {
				return err
			}
		}

		gidint = int(i)
	}

	// chown logic
	fi, err := os.Lstat(filepath)
	if err != nil {
		return err
	}

	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		if err := os.Lchown(filepath, uidint, gidint); err != nil {
			return err
		}
	} else {
		if err := os.Chown(filepath, uidint, gidint); err != nil {
			return err
		}
	}

	return nil
}
