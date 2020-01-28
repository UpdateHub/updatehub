// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, download_abort, SharedState},
    Download, Result, State, StateChangeImpl, StateMachine,
};
use crate::{
    client::Api,
    firmware::installation_set,
    object::{self, Info},
    update_package::UpdatePackage,
};
use slog_scope::error;
use std::{fs, sync::mpsc};
use walkdir::WalkDir;

#[derive(Debug, PartialEq)]
pub(super) struct PrepareDownload {
    pub(super) update_package: UpdatePackage,
}

#[async_trait::async_trait]
impl StateChangeImpl for State<PrepareDownload> {
    fn name(&self) -> &'static str {
        "prepare_download"
    }

    fn handle_download_abort(&self) -> download_abort::Response {
        download_abort::Response::RequestAccepted
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition)> {
        crate::logger::buffer().lock().unwrap().start_logging();
        let installation_set = installation_set::inactive()?;
        let download_dir = shared_state.settings.update.download_dir.to_owned();

        // Prune left over from previous installations
        for entry in WalkDir::new(&download_dir)
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
            &shared_state.settings,
            installation_set,
            object::info::Status::Corrupted,
        ) {
            fs::remove_file(download_dir.join(object.sha256sum()))?;
        }

        // Get shasums of missing or incomplete objects
        let shasum_list: Vec<_> = self
            .0
            .update_package
            .objects(installation_set)
            .iter()
            .filter(|o| {
                let obj_status = o
                    .status(&download_dir)
                    .map_err(|e| {
                        error!("Fail accessing the object: {} (err: {})", o.sha256sum(), e)
                    })
                    .unwrap_or(object::info::Status::Missing);
                obj_status == object::info::Status::Missing
                    || obj_status == object::info::Status::Incomplete
            })
            .map(|obj| obj.sha256sum().to_owned())
            .collect();

        // Get ownership of remaining data that will be sent to new thread
        let server = shared_state.server_address().to_owned();
        let product_uid = shared_state.firmware.product_uid.to_owned();
        let package_uid = self.0.update_package.package_uid();
        let (sndr, recv) = mpsc::channel();

        // Download the missing or incomplete objects
        actix::Arbiter::spawn(async move {
            let api = Api::new(&server);
            let mut results = Vec::default();
            for shasum in shasum_list.iter() {
                results.push(
                    api.download_object(&product_uid, &package_uid, &download_dir, &shasum).await,
                );
            }
            sndr.send(results).expect("Unable to send response about object downlod");
        });

        Ok((
            StateMachine::Download(State(Download {
                update_package: self.0.update_package,
                installation_set,
                download_chan: recv,
            })),
            actor::StepTransition::Immediate,
        ))
    }
}
