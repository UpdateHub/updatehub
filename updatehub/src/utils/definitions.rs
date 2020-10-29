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

/// Utility functions for [TargetType](pkg_schema::definitions::TargetType)
pub(crate) trait TargetTypeExt {
    /// Checks whether the device is valid to start installation, i.e.,
    /// device exists, use have write permission.
    fn valid(&self) -> Result<&Self>;

    /// Gets device's path for mounting.
    fn get_target(&self) -> Result<PathBuf>;
}

impl TargetTypeExt for TargetType {
    fn valid(&self) -> Result<&Self> {
        let device = self.get_target()?;

        if !device.exists() {
            return Err(Error::DeviceDoesNotExist(device));
        }

        if device.metadata()?.permissions().readonly() {
            return Err(Error::MissingWritePermission(device));
        }

        Ok(&self)
    }

    fn get_target(&self) -> Result<PathBuf> {
        match self {
            TargetType::Device(p) => Ok(p.clone()),
            TargetType::UBIVolume(s) => mtd::target_device_from_ubi_volume_name(s),
            TargetType::MTDName(s) => mtd::target_device_from_mtd_name(s),
        }
    }
}

/// Utility functions for [Gid](pkg_schema::definitions::target_permissions::Gid)
/// and [Uid](pkg_schema::definitions::target_permissions::Uid)
pub(crate) trait IdExt {
    /// Gets numeric id;
    fn as_u32(&self) -> u32;
}

impl IdExt for Gid {
    fn as_u32(&self) -> u32 {
        match self {
            Gid::Name(s) => nix::unistd::Group::from_name(s).unwrap().unwrap().gid.as_raw(),
            Gid::Number(n) => *n,
        }
    }
}

impl IdExt for Uid {
    fn as_u32(&self) -> u32 {
        match self {
            Uid::Name(s) => nix::unistd::User::from_name(s).unwrap().unwrap().uid.as_raw(),
            Uid::Number(n) => *n,
        }
    }
}
