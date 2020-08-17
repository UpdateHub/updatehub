// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

mod supported_hardware;

use self::supported_hardware::SupportedHardwareExt;
use crate::{
    firmware::{installation_set::Set, Metadata},
    object::{self, Info},
    settings::Settings,
};
use derive_more::{Display, Error, From};
use pkg_schema::Object;
use sdk::api::info::runtime_settings::InstallationSet;
use slog_scope::error;
use std::{fs, io, path::Path};
use walkdir::WalkDir;

#[cfg(test)]
pub(crate) mod tests;

pub(crate) use cloud::api::{Signature, UpdatePackage};

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, Error, From)]
pub enum Error {
    Io(std::io::Error),
    CloudSDK(cloud::Error),

    #[from(ignore)]
    IncompatibleHardware(#[error(not(source))] String),
    #[from(ignore)]
    #[display(fmt = "Install mode not accepted: {}", _0)]
    IncompatibleInstallMode(#[error(not(source))] String),
}

pub(crate) trait UpdatePackageExt {
    fn compatible_with(&self, firmware: &Metadata) -> Result<()>;

    fn validate_install_modes(&self, settings: &Settings, installation_set: Set) -> Result<()>;

    fn objects(&self, installation_set: Set) -> &Vec<Object>;

    fn objects_mut(&mut self, installation_set: Set) -> &mut Vec<Object>;

    fn filter_objects(
        &self,
        settings: &Settings,
        installation_set: Set,
        filter: object::info::Status,
    ) -> Vec<&Object>;

    fn clear_unrelated_files(
        &self,
        dir: &Path,
        installation_set: Set,
        settings: &Settings,
    ) -> io::Result<()>;
}

impl UpdatePackageExt for UpdatePackage {
    fn compatible_with(&self, firmware: &Metadata) -> Result<()> {
        self.inner.supported_hardware.compatible_with(&firmware.hardware)
    }

    fn validate_install_modes(&self, settings: &Settings, installation_set: Set) -> Result<()> {
        let install_modes = &settings.update.supported_install_modes;
        if let Some(mode) = self
            .objects(installation_set)
            .iter()
            .map(|o| o.mode())
            .find(|mode| !install_modes.contains(&mode))
        {
            return Err(Error::IncompatibleInstallMode(mode));
        }

        Ok(())
    }

    fn objects(&self, installation_set: Set) -> &Vec<Object> {
        match installation_set.0 {
            InstallationSet::A => &self.inner.objects.0,
            InstallationSet::B => &self.inner.objects.1,
        }
    }

    fn objects_mut(&mut self, installation_set: Set) -> &mut Vec<Object> {
        match installation_set.0 {
            InstallationSet::A => &mut self.inner.objects.0,
            InstallationSet::B => &mut self.inner.objects.1,
        }
    }

    fn filter_objects(
        &self,
        settings: &Settings,
        installation_set: Set,
        filter: object::info::Status,
    ) -> Vec<&Object> {
        self.objects(installation_set)
            .iter()
            .filter(|o| {
                o.status(&settings.update.download_dir)
                    .map_err(|e| {
                        error!("fail accessing the object: {} (err: {})", o.sha256sum(), e)
                    })
                    .unwrap_or(object::info::Status::Missing)
                    .eq(&filter)
            })
            .collect()
    }

    fn clear_unrelated_files(
        &self,
        dir: &Path,
        installation_set: Set,
        settings: &Settings,
    ) -> io::Result<()> {
        // Prune left over objects from previous installations
        for entry in WalkDir::new(dir)
            .follow_links(true)
            .min_depth(1)
            .into_iter()
            .filter_entry(|e| e.file_type().is_file())
            .filter_map(std::result::Result::ok)
            .filter(|e| {
                !self
                    .objects(installation_set)
                    .iter()
                    .map(object::Info::sha256sum)
                    .any(|x| x == e.file_name())
            })
        {
            fs::remove_file(entry.path())?;
        }

        // Cleanup metadata and signature for older local local installation
        for file in &[dir.join("metadata"), dir.join("signature")] {
            if file.exists() {
                fs::remove_file(file)?;
            }
        }

        // Prune corrupted files
        for object in
            self.filter_objects(&settings, installation_set, object::info::Status::Corrupted)
        {
            fs::remove_file(dir.join(object.sha256sum()))?;
        }

        Ok(())
    }
}
