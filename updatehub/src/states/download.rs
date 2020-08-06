// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    machine::{self, CommunicationState, Context},
    Install, ProgressReporter, Result, State, StateChangeImpl, TransitionError,
};
use crate::{
    firmware::installation_set,
    object::{self, Info},
    update_package::{UpdatePackage, UpdatePackageExt},
};
use async_lock::Lock;
use async_std::prelude::FutureExt;
use slog_scope::{debug, error, trace};

#[derive(Debug)]
pub(super) struct Download {
    pub(super) update_package: UpdatePackage,
}

impl Download {
    async fn start_download(&self, context: &Lock<&mut Context>) -> Result<()> {
        let installation_set = installation_set::inactive()?;
        let download_dir = context.lock().await.settings.update.download_dir.to_owned();

        self.update_package.clear_unrelated_files(
            &download_dir,
            installation_set,
            &context.lock().await.settings,
        )?;

        // Get shasums of missing or incomplete objects
        let shasum_list: Vec<_> = self
            .update_package
            .objects(installation_set)
            .iter()
            .filter_map(|o| {
                let name = o.filename();
                let shasum = o.sha256sum();
                let obj_status = o
                    .status(&download_dir)
                    .map_err(|e| {
                        error!("fail accessing the object: {} ({}) (err: {})", name, shasum, e)
                    })
                    .unwrap_or(object::info::Status::Missing);
                if obj_status == object::info::Status::Missing
                    || obj_status == object::info::Status::Incomplete
                {
                    Some((name.to_owned(), shasum.to_owned()))
                } else {
                    debug!("skiping object: {} ({})", name, shasum);
                    None
                }
            })
            .collect();

        trace!("the following objects are missing: {:?}", shasum_list);

        // Download the missing or incomplete objects
        let url = context.lock().await.server_address().to_owned();
        let product_uid = context.lock().await.firmware.product_uid.clone();
        let api = crate::CloudClient::new(&url);
        for (name, shasum) in shasum_list.into_iter() {
            debug!("starting download of: {} ({})", name, shasum);
            api.download_object(
                &product_uid,
                &self.update_package.package_uid(),
                &download_dir,
                &shasum,
            )
            .await?;
        }

        Ok(())
    }
}

#[async_trait::async_trait(?Send)]
impl ProgressReporter for Download {
    fn package_uid(&self) -> String {
        self.update_package.package_uid()
    }

    fn report_enter_state_name(&self) -> &'static str {
        "downloading"
    }

    fn report_leave_state_name(&self) -> &'static str {
        "downloaded"
    }
}

impl CommunicationState for Download {}

#[async_trait::async_trait(?Send)]
impl StateChangeImpl for Download {
    fn name(&self) -> &'static str {
        "download"
    }

    fn is_handling_download(&self) -> bool {
        true
    }

    async fn handle(mut self, context: &mut Context) -> Result<(State, machine::StepTransition)> {
        use std::ops::DerefMut;
        let communication_receiver = &context.communication.receiver.clone();
        let context = Lock::new(context);

        let download_future = async {
            self.start_download(&context).await?;
            Result::Ok(None)
        };

        let message_handle_future = async {
            while let Ok((msg, responder)) = communication_receiver.recv().await {
                if let Some(new_state) = self
                    .handle_communication(msg, responder, context.lock().await.deref_mut())
                    .await
                {
                    return Ok(Some(new_state));
                }
            }
            Ok(None)
        };

        if let Some(new_state) = download_future.race(message_handle_future).await? {
            return Ok((new_state, machine::StepTransition::Immediate));
        }

        let download_dir = &context.lock().await.settings.update.download_dir;
        if self
            .update_package
            .objects(installation_set::inactive()?)
            .iter()
            .all(|o| o.status(download_dir).ok() == Some(object::info::Status::Ready))
        {
            Ok((
                State::Install(Install { update_package: self.update_package }),
                machine::StepTransition::Immediate,
            ))
        } else {
            Err(TransitionError::ObjectsNotReady)
        }
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use crate::{cloud_mock, update_package::tests::get_update_package_with_shasum, utils};
    use pretty_assertions::assert_eq;
    use std::{fs, io::Read};
    use walkdir::WalkDir;

    fn fake_download_object(size: usize) -> (Vec<u8>, String) {
        let vec = std::iter::repeat(0xF).take(size).collect::<Vec<_>>();
        let shasum = utils::sha256sum(&vec);
        (vec, shasum)
    }

    async fn test_object_download(size: usize) {
        let setup = crate::tests::TestEnvironment::build().finish();
        let mut context = setup.gen_context();
        let (obj, shasum) = fake_download_object(size);
        let download_state = Download { update_package: get_update_package_with_shasum(&shasum) };
        let download_dir = context.settings.update.download_dir.clone();

        // leftover file to ensure it is removed
        fs::File::create(&download_dir.join("leftover-file")).unwrap();

        cloud_mock::set_download_data(obj);

        let machine =
            State::Download(download_state).move_to_next_state(&mut context).await.unwrap().0;
        assert_state!(machine, Install);

        assert_eq!(
            WalkDir::new(&download_dir)
                .follow_links(true)
                .min_depth(1)
                .into_iter()
                .filter_entry(|e| e.file_type().is_file())
                .count(),
            1,
            "Failed to remove the corrupted object"
        );

        let mut object_content = String::new();
        let _ = fs::File::open(&download_dir.join(&shasum))
            .expect("Fail to open the temporary directory")
            .read_to_string(&mut object_content);

        assert_eq!(&utils::sha256sum(&object_content.as_bytes()), &shasum, "Checksum mismatch");
    }

    #[async_std::test]
    #[ignore]
    async fn download_small_object() {
        test_object_download(16).await
    }

    #[async_std::test]
    #[ignore]
    async fn download_large_object() {
        test_object_download(100_000_000).await
    }
}
