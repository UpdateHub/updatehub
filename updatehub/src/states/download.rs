// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    machine::{self, SharedState},
    Install, ProgressReporter, Result, State, StateChangeImpl, TransitionError,
};
use crate::{
    firmware::installation_set,
    object::{self, Info},
    update_package::{UpdatePackage, UpdatePackageExt},
};
use std::fmt;

pub(super) struct Download {
    pub(super) update_package: UpdatePackage,
    pub(super) installation_set: installation_set::Set,
    pub(super) download_chan: tokio::sync::mpsc::Receiver<Vec<cloud::Result<()>>>,
}

impl PartialEq for Download {
    fn eq(&self, other: &Self) -> bool {
        // download_chan intentionally ignored
        self.update_package == other.update_package
            && self.installation_set == other.installation_set
    }
}

impl fmt::Debug for Download {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        // download_chan intentionally ignored
        write!(
            f,
            "Download {{ update_package: {:?}, installation_set: {:?} }}",
            self.update_package, self.installation_set
        )
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

#[async_trait::async_trait(?Send)]
impl StateChangeImpl for Download {
    fn name(&self) -> &'static str {
        "download"
    }

    fn is_handling_download(&self) -> bool {
        true
    }

    async fn handle(
        mut self,
        shared_state: &mut SharedState,
    ) -> Result<(State, machine::StepTransition)> {
        if let Some(vec) = self.download_chan.recv().await {
            vec.into_iter().try_for_each(|res| res)?;
        }

        let download_dir = &shared_state.settings.update.download_dir;
        if self
            .update_package
            .objects(self.installation_set)
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
    use crate::{
        cloud_mock, states::PrepareDownload, update_package::tests::get_update_package_with_shasum,
        utils,
    };
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
        let mut shared_state = setup.gen_shared_state();
        let (obj, shasum) = fake_download_object(size);
        let predownload_state =
            PrepareDownload { update_package: get_update_package_with_shasum(&shasum) };
        let download_dir = shared_state.settings.update.download_dir.clone();

        // leftover file to ensure it is removed
        fs::File::create(&download_dir.join("leftover-file")).unwrap();

        cloud_mock::set_download_data(obj);

        let mut machine = State::PrepareDownload(predownload_state)
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;
        assert_state!(machine, Download);
        loop {
            machine = machine.move_to_next_state(&mut shared_state).await.unwrap().0;
            if let State::Install(_) = machine {
                break;
            }
        }
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
            .expect("Fail to open the temporary directory.")
            .read_to_string(&mut object_content);

        assert_eq!(&utils::sha256sum(&object_content.as_bytes()), &shasum, "Checksum mismatch");
    }

    #[actix_rt::test]
    #[ignore]
    async fn download_small_object() {
        test_object_download(16).await
    }

    #[actix_rt::test]
    #[ignore]
    async fn download_large_object() {
        test_object_download(100_000_000).await
    }
}
