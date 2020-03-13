// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Install, Result, State, StateChangeImpl, StateMachine,
};
use crate::{firmware::installation_set, update_package::UpdatePackage};
use slog_scope::{debug, info};
use std::{fs, path::PathBuf};

#[derive(Debug, PartialEq)]
pub(super) struct PrepareLocalInstall {
    pub(super) update_file: PathBuf,
}

#[async_trait::async_trait]
impl StateChangeImpl for State<PrepareLocalInstall> {
    fn name(&self) -> &'static str {
        "prepare_local_install"
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition)> {
        info!("Prepare local install: {:?}", self.0.update_file);
        let dest_path = shared_state.settings.update.download_dir.clone();
        std::fs::create_dir_all(&dest_path)?;
        compress_tools::uncompress(self.0.update_file, &dest_path, compress_tools::Kind::Zip)
            .map_err(super::TransitionError::Uncompress)?;
        debug!("Successfuly uncompressed the update package");

        let metadata = fs::read(dest_path.join("metadata"))?;
        let update_package = UpdatePackage::parse(&metadata)?;
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
