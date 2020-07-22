// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    machine::{self, Context},
    Download, Result, State, StateChangeImpl,
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
impl StateChangeImpl for PrepareDownload {
    fn name(&self) -> &'static str {
        "prepare_download"
    }

    fn is_handling_download(&self) -> bool {
        true
    }

    async fn handle(self, context: &mut Context) -> Result<(State, machine::StepTransition)> {
        let installation_set = installation_set::inactive()?;
        let download_dir = context.settings.update.download_dir.to_owned();

        self.update_package.clear_unrelated_files(
            &download_dir,
            installation_set,
            &context.settings,
        )?;

        // Get shasums of missing or incomplete objects
        let shasum_list: Vec<_> = self
            .update_package
            .objects(installation_set)
            .iter()
            .filter(|o| {
                let obj_status = o
                    .status(&download_dir)
                    .map_err(|e| {
                        error!("fail accessing the object: {} (err: {})", o.sha256sum(), e)
                    })
                    .unwrap_or(object::info::Status::Missing);
                obj_status == object::info::Status::Missing
                    || obj_status == object::info::Status::Incomplete
            })
            .map(|obj| obj.sha256sum().to_owned())
            .collect();

        // Get ownership of remaining data that will be sent to new thread
        let server = context.server_address().to_owned();
        let product_uid = context.firmware.product_uid.to_owned();
        let package_uid = self.update_package.package_uid();
        let (sndr, recv) = async_std::sync::channel(1);

        // Download the missing or incomplete objects
        async_std::task::spawn_local(async move {
            let api = crate::CloudClient::new(&server);
            let mut results = Vec::default();
            for shasum in shasum_list.iter() {
                results.push(
                    api.download_object(&product_uid, &package_uid, &download_dir, &shasum).await,
                );
            }
            sndr.send(results).await;
        });

        Ok((
            State::Download(Download {
                update_package: self.update_package,
                installation_set,
                download_chan: recv,
            }),
            machine::StepTransition::Immediate,
        ))
    }
}
