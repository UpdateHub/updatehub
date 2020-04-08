// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Install, Result, State, StateChangeImpl, StateMachine,
};
use crate::{
    firmware::installation_set,
    update_package::{Signature, UpdatePackage, UpdatePackageExt},
};
use slog_scope::{debug, info};
use std::{
    fs,
    io::{Seek, SeekFrom},
    path::PathBuf,
};

#[derive(Debug, PartialEq)]
pub(super) struct PrepareLocalInstall {
    pub(super) update_file: PathBuf,
}

#[async_trait::async_trait(?Send)]
impl StateChangeImpl for State<PrepareLocalInstall> {
    fn name(&self) -> &'static str {
        "prepare_local_install"
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition)> {
        info!("Prepare local install: {}", self.0.update_file.display());
        let dest_path = shared_state.settings.update.download_dir.clone();
        std::fs::create_dir_all(&dest_path)?;

        let mut metadata = Vec::with_capacity(1024);
        let mut source = fs::File::open(self.0.update_file)?;
        compress_tools::uncompress_archive_file(&mut source, &mut metadata, "metadata")?;
        let update_package = UpdatePackage::parse(&metadata)?;
        debug!("Successfuly uncompressed metadata file");

        if let Some(key) = shared_state.firmware.pub_key.as_ref() {
            let mut sign = Vec::with_capacity(512);
            source.seek(SeekFrom::Start(0))?;
            match compress_tools::uncompress_archive_file(&mut source, &mut sign, "signature") {
                Ok(()) => {
                    let sign = String::from_utf8(sign)?;
                    let sign = Signature::from_base64_str(&sign)?;
                    debug!("Validating signature");
                    sign.validate(key, &update_package)?;
                }
                Err(compress_tools::Error::FileNotFound) => {}
                Err(e) => return Err(e.into()),
            }
        }

        for object in update_package
            .objects(installation_set::active()?)
            .iter()
            .map(crate::object::Info::sha256sum)
        {
            source.seek(SeekFrom::Start(0))?;

            let mut target = fs::File::create(dest_path.join(object))?;
            compress_tools::uncompress_archive_file(&mut source, &mut target, object)?;
        }

        debug!("Update package extracted: {:?}", update_package);

        update_package.clear_unrelated_files(
            &dest_path,
            installation_set::inactive()?,
            &shared_state.settings,
        )?;

        Ok((
            StateMachine::Install(State(Install { update_package })),
            actor::StepTransition::Immediate,
        ))
    }
}
