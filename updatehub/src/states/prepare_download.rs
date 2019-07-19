// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{actor::download_abort, Download, State, StateChangeImpl, StateMachine};
use crate::{
    firmware::installation_set,
    object::{self, Info},
    update_package::UpdatePackage,
};

use std::fs;
use walkdir::WalkDir;

#[derive(Debug, PartialEq)]
pub(super) struct PrepareDownload {
    pub(super) update_package: UpdatePackage,
}

impl StateChangeImpl for State<PrepareDownload> {
    fn name(&self) -> &'static str {
        "prepare_download"
    }

    fn handle_download_abort(&self) -> download_abort::Response {
        download_abort::Response::RequestAccepted
    }

    fn handle(self) -> Result<StateMachine, failure::Error> {
        crate::logger::buffer().lock().unwrap().start_logging();
        let installation_set = installation_set::inactive()?;
        let download_dir = &shared_state!().settings.update.download_dir.clone();

        // Prune left over from previous installations
        for entry in WalkDir::new(download_dir)
            .follow_links(true)
            .min_depth(1)
            .into_iter()
            .filter_entry(|e| e.file_type().is_file())
            .filter_map(std::result::Result::ok)
            .filter(|e| {
                !self
                    .0
                    .update_package
                    .objects(installation_set)
                    .iter()
                    .map(object::Info::sha256sum)
                    .any(|x| x == e.file_name())
            })
        {
            fs::remove_file(entry.path())?;
        }

        // Prune corrupted files
        for object in self.0.update_package.filter_objects(
            &shared_state!().settings,
            installation_set,
            object::info::Status::Corrupted,
        ) {
            fs::remove_file(download_dir.join(object.sha256sum()))?;
        }

        Ok(StateMachine::Download(State(Download {
            update_package: self.0.update_package,
            installation_set,
        })))
    }
}
