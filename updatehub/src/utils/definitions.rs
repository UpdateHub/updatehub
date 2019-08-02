// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::utils::mtd;
use failure::ensure;
use pkg_schema::definitions::{
    target_permissions::{Gid, Uid},
    TargetType,
};
use std::path::PathBuf;

/// Utility funtions for [TargetType](pkg_schema::definitions::TargetType)
pub(crate) trait TargetTypeExt {
    /// Checks whether the device is valid to start installation, i.e.,
    /// device exists, use have write permission.
    fn valid(&self) -> Result<&Self, failure::Error>;

    /// Gets device's path for mounting.
    fn get_target(&self) -> Result<PathBuf, failure::Error>;
}

impl TargetTypeExt for TargetType {
    fn valid(&self) -> Result<&Self, failure::Error> {
        Ok(match self {
            TargetType::Device(p) => {
                ensure!(p.exists(), "Target device does not exists");
                ensure!(
                    !p.metadata()?.permissions().readonly(),
                    "User doesn't have write permission on target device: {:?}",
                    p
                );
                &self
            }
            TargetType::UBIVolume(s) => {
                let dev = mtd::target_device_from_ubi_volume_name(s)?;
                ensure!(
                    !dev.metadata()?.permissions().readonly(),
                    "User doesn't have write permission on target device: {:?}",
                    dev
                );
                &self
            }
            TargetType::MTDName(_) => unimplemented!("FIXME: Check if MTD name is valid"),
        })
    }

    fn get_target(&self) -> Result<PathBuf, failure::Error> {
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
                let s = std::ffi::CString::new(s.as_str());
                unsafe { *nix::libc::getgrnam(s.unwrap().as_ptr()) }.gr_gid
            }
            Gid::Number(n) => *n,
        }
    }
}

impl IdExt for Uid {
    fn as_u32(&self) -> u32 {
        match self {
            Uid::Name(s) => {
                let s = std::ffi::CString::new(s.as_str());
                unsafe { *nix::libc::getpwnam(s.unwrap().as_ptr()) }.pw_uid
            }
            Uid::Number(n) => *n,
        }
    }
}
