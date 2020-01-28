// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Error, Result};
use crate::utils::mtd;
use pkg_schema::definitions::{
    target_permissions::{Gid, Uid},
    TargetType,
};
use std::path::PathBuf;

/// Utility funtions for [TargetType](pkg_schema::definitions::TargetType)
pub(crate) trait TargetTypeExt {
    /// Checks whether the device is valid to start installation, i.e.,
    /// device exists, use have write permission.
    fn valid(&self) -> Result<&Self>;

    /// Gets device's path for mounting.
    fn get_target(&self) -> Result<PathBuf>;
}

impl TargetTypeExt for TargetType {
    fn valid(&self) -> Result<&Self> {
        Ok(match self {
            TargetType::Device(p) => {
                if !p.exists() {
                    return Err(Error::DeviceDoesNotExist);
                }
                if p.metadata()?.permissions().readonly() {
                    return Err(Error::MissingWritePermission(p.to_path_buf()));
                }
                &self
            }
            TargetType::UBIVolume(s) => {
                let dev = mtd::target_device_from_ubi_volume_name(s)?;
                if dev.metadata()?.permissions().readonly() {
                    return Err(Error::MissingWritePermission(dev));
                }
                &self
            }
            TargetType::MTDName(n) => {
                let dev = mtd::target_device_from_mtd_name(n)?;
                if dev.metadata()?.permissions().readonly() {
                    return Err(Error::MissingWritePermission(dev));
                }
                &self
            }
        })
    }

    fn get_target(&self) -> Result<PathBuf> {
        match self {
            TargetType::Device(p) => Ok(p.clone()),
            TargetType::UBIVolume(s) => mtd::target_device_from_ubi_volume_name(s),
            TargetType::MTDName(s) => mtd::target_device_from_mtd_name(s),
        }
    }
}

/// Utility funtions for [Gid](pkg_schema::definitions::target_permissions::Gid)
/// and [Uid](pkg_schema::definitions::target_permissions::Uid)
pub(crate) trait IdExt {
    /// Gets numeric id;
    fn as_u32(&self) -> u32;
}

impl IdExt for Gid {
    fn as_u32(&self) -> u32 {
        match self {
            Gid::Name(s) => {
                let s = std::ffi::CString::new(s.as_str()).unwrap();
                unsafe { *nix::libc::getgrnam(s.as_ptr()) }.gr_gid
            }
            Gid::Number(n) => *n,
        }
    }
}

impl IdExt for Uid {
    fn as_u32(&self) -> u32 {
        match self {
            Uid::Name(s) => {
                let s = std::ffi::CString::new(s.as_str()).unwrap();
                unsafe { *nix::libc::getpwnam(s.as_ptr()) }.pw_uid
            }
            Uid::Number(n) => *n,
        }
    }
}
