// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    machine::{self, Context},
    CallbackReporter, Result, State, StateChangeImpl, Validation,
};
use crate::{
    firmware::installation_set,
    update_package::{Signature, UpdatePackage, UpdatePackageExt},
    utils::log::LogContent,
};
use slog_scope::{debug, error, info};
use std::{
    fs,
    io::{self, Seek, SeekFrom},
    path::PathBuf,
    str,
};

#[derive(Debug)]
pub(super) struct PrepareLocalInstall {
    pub(super) update_file: PathBuf,
}

impl CallbackReporter for PrepareLocalInstall {}

#[async_trait::async_trait(?Send)]
impl StateChangeImpl for PrepareLocalInstall {
    fn name(&self) -> &'static str {
        "prepare_local_install"
    }

    async fn handle(self, context: &mut Context) -> Result<(State, machine::StepTransition)> {
        info!("installing local package: {:?}", self.update_file);
        let dest_path = context.settings.update.download_dir.clone();
        std::fs::create_dir_all(&dest_path).log_error_msg("unable to create download dir")?;

        let mut metadata = Vec::with_capacity(1024);
        let mut source = fs::File::open(self.update_file).log_error_msg("unable to open uhupkg")?;
        compress_tools::uncompress_archive_file(&mut source, &mut metadata, "metadata")
            .log_error_msg("failed to uncompress metadata from uhupkg")?;
        let update_package =
            UpdatePackage::parse(&metadata).log_error_msg("failed to parse extracted metadata")?;
        debug!("successfuly uncompressed metadata file");

        let sign = {
            let mut sign = Vec::with_capacity(512);
            source
                .seek(SeekFrom::Start(0))
                .log_error_msg("failed to seek uhupkg back to the start")?;
            match compress_tools::uncompress_archive_file(&mut source, &mut sign, "signature") {
                Ok(_) => {
                    let sign = Signature::from_base64_str(
                        str::from_utf8(&sign)
                            .log_error_msg("failed to parse utf8 from signature")?,
                    )
                    .log_error_msg("failed to parse base64 from signature")?;
                    Some(sign)
                }
                Err(compress_tools::Error::Io(e)) if e.kind() == io::ErrorKind::NotFound => {
                    error!("package does not contain a signature file");
                    return Err(super::TransitionError::SignatureNotFound);
                }
                Err(e) => return Err(e.into()),
            }
        };

        for object in update_package
            .objects(
                installation_set::active()
                    .log_error_msg("failed to get active installation set")?,
            )
            .iter()
            // We ignore object's allow_remote_install property since we are doing
            // a local install and hence offline update is implied
            .map(crate::object::Info::sha256sum)
        {
            source.seek(SeekFrom::Start(0)).log_error_msg("failed to seek uhupkg to the start")?;

            let mut target = fs::File::create(dest_path.join(object))
                .log_error_msg("failed to create output file for object")?;
            compress_tools::uncompress_archive_file(&mut source, &mut target, object)
                .log_error_msg("failed to uncompress object")?;
        }

        update_package
            .clear_unrelated_files(
                &dest_path,
                installation_set::inactive()
                    .log_error_msg("failed to get inactive installation set")?,
                &context.settings,
            )
            .log_error_msg("unable to cleanup unrequired files from download dir")?;

        info!(
            "update package extracted: {} ({})",
            update_package.version(),
            update_package.package_uid()
        );
        Ok((
            State::Validation(Validation {
                package: update_package,
                sign,
                require_download: false,
            }),
            machine::StepTransition::Immediate,
        ))
    }
}
