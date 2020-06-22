// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, SharedState},
    Download, Result, State, StateChangeImpl, StateMachine,
};
use crate::{
    firmware::installation_set,
    object::{self, Info},
    update_package::{UpdatePackage, UpdatePackageExt},
};
use slog_scope::error;

#[derive(Debug, PartialEq)]
pub(super) struct PrepareDownload {
    pub(super) update_package: UpdatePackage,
}

#[async_trait::async_trait(?Send)]
impl StateChangeImpl for State<PrepareDownload> {
    fn name(&self) -> &'static str {
        "prepare_download"
    }

    fn can_run_download_abort(&self) -> bool {
        true
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition)> {
        let installation_set = installation_set::inactive()?;
        let download_dir = shared_state.settings.update.download_dir.to_owned();

        self.0.update_package.clear_unrelated_files(
            &download_dir,
            installation_set,
            &shared_state.settings,
        )?;

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
        let (mut sndr, recv) = tokio::sync::mpsc::channel(1);

        // Download the missing or incomplete objects
        actix::Arbiter::spawn(async move {
            let api = crate::CloudClient::new(&server);
            let mut results = Vec::default();
            for shasum in shasum_list.iter() {
                results.push(
                    api.download_object(&product_uid, &package_uid, &download_dir, &shasum).await,
                );
            }
            sndr.send(results).await.expect("Unable to send response about object downlod");
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
