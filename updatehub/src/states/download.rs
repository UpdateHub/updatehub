// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::{self, download_abort, SharedState},
    Idle, Install, ProgressReporter, State, StateChangeImpl, StateMachine, TransitionCallback,
};
use crate::{
    firmware::installation_set,
    object::{self, Info},
    update_package::UpdatePackage,
};
use derivative::Derivative;
use failure::format_err;
use std::sync::mpsc;

#[derive(Derivative)]
#[derivative(Debug, PartialEq)]
pub(super) struct Download {
    pub(super) update_package: UpdatePackage,
    pub(super) installation_set: installation_set::Set,
    #[derivative(PartialEq = "ignore")]
    #[derivative(Debug = "ignore")]
    pub(super) download_chan: mpsc::Receiver<Vec<Result<(), failure::Error>>>,
}

create_state_step!(Download => Idle);
create_state_step!(Download => Install(update_package));

impl TransitionCallback for State<Download> {}

impl ProgressReporter for State<Download> {
    fn package_uid(&self) -> String {
        self.0.update_package.package_uid()
    }

    fn report_enter_state_name(&self) -> &'static str {
        "downloading"
    }

    fn report_leave_state_name(&self) -> &'static str {
        "downloaded"
    }
}

#[async_trait::async_trait]
impl StateChangeImpl for State<Download> {
    fn name(&self) -> &'static str {
        "download"
    }

    fn handle_download_abort(&self) -> download_abort::Response {
        download_abort::Response::RequestAccepted
    }

    async fn handle(
        self,
        shared_state: &mut SharedState,
    ) -> Result<(StateMachine, actor::StepTransition), failure::Error> {
        match self.0.download_chan.try_recv() {
            Ok(vec) => vec.into_iter().try_for_each(|res| res)?,
            Err(mpsc::TryRecvError::Empty) => {
                return Ok((StateMachine::Download(self), actor::StepTransition::Immediate));
            }
            Err(e) => return Err(format_err!("Failed to read from channel: {:?}", e)),
        }

        let download_dir = &shared_state.settings.update.download_dir;
        if self
            .0
            .update_package
            .objects(self.0.installation_set)
            .iter()
            .all(|o| o.status(download_dir).ok() == Some(object::info::Status::Ready))
        {
            Ok((StateMachine::Install(self.into()), actor::StepTransition::Immediate))
        } else {
            Err(format_err!("Not all objects are ready for use"))
        }
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use crate::{
        firmware::{
            tests::{create_fake_installation_set, create_fake_metadata, FakeDevice},
            Metadata,
        },
        runtime_settings::RuntimeSettings,
        states::PrepareDownload,
        update_package::tests::{create_fake_settings, get_update_package_with_shasum},
    };
    use crypto_hash::{hex_digest, Algorithm};
    use mockito::mock;
    use pretty_assertions::assert_eq;
    use std::{
        env,
        fs::{create_dir_all, File},
        io::Read,
    };
    use walkdir::WalkDir;

    fn fake_download_object(size: usize) -> (Vec<u8>, String) {
        let vec = std::iter::repeat(0xF).take(size).collect::<Vec<_>>();
        let shasum = hex_digest(Algorithm::SHA256, &vec);
        (vec, shasum)
    }

    fn fake_download_state(shasum: &str) -> (State<PrepareDownload>, SharedState) {
        let settings = create_fake_settings();
        let tmpdir = settings.update.download_dir.clone();
        let _ = create_dir_all(&tmpdir);
        create_fake_installation_set(&tmpdir, 0);
        env::set_var("PATH", format!("{}", &tmpdir.to_string_lossy()));
        let runtime_settings = RuntimeSettings::default();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();

        (
            State(PrepareDownload { update_package: get_update_package_with_shasum(shasum) }),
            SharedState { settings, runtime_settings, firmware },
        )
    }

    async fn test_object_download(size: usize) {
        let (obj, shasum) = fake_download_object(size);
        let (predownload_state, mut shared_state) = fake_download_state(&shasum);
        let tmpdir = shared_state.settings.update.download_dir.clone();

        // leftover file to ensure it is removed
        let _ = File::create(&tmpdir.join("leftover-file"));

        let mock = mock(
            "GET",
            format!(
                "/products/{}/packages/{}/objects/{}",
                "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                &predownload_state.0.update_package.package_uid(),
                &shasum
            )
            .as_str(),
        )
        .match_header("Content-Type", "application/json")
        .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
        .with_status(200)
        .with_body(obj)
        .create();

        let mut machine = StateMachine::PrepareDownload(predownload_state)
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;
        assert_state!(machine, Download);
        loop {
            machine = machine.move_to_next_state(&mut shared_state).await.unwrap().0;
            if let StateMachine::Install(_) = machine {
                break;
            }
        }
        assert_state!(machine, Install);

        mock.assert();

        assert_eq!(
            WalkDir::new(&tmpdir)
                .follow_links(true)
                .min_depth(1)
                .into_iter()
                .filter_entry(|e| e.file_type().is_file())
                .count(),
            1,
            "Failed to remove the corrupted object"
        );

        let mut object_content = String::new();
        let _ = File::open(&tmpdir.join(&shasum))
            .expect("Fail to open the temporary directory.")
            .read_to_string(&mut object_content);

        assert_eq!(
            &hex_digest(Algorithm::SHA256, object_content.as_bytes()),
            &shasum,
            "Checksum mismatch"
        );
    }

    #[actix_rt::test]
    async fn skip_download_if_ready() {
        use crate::update_package::tests::create_fake_object;
        let (obj, shasum) = fake_download_object(16);
        let (predownload_state, mut shared_state) = fake_download_state(&shasum);
        let tmpdir = shared_state.settings.update.download_dir.clone();

        create_fake_object(&obj, &shasum, &shared_state.settings);

        let machine = StateMachine::PrepareDownload(predownload_state)
            .move_to_next_state(&mut shared_state)
            .await
            .unwrap()
            .0;
        assert_state!(machine, Download);
        let machine = machine.move_to_next_state(&mut shared_state).await.unwrap().0;
        assert_state!(machine, Install);

        assert_eq!(
            WalkDir::new(&tmpdir)
                .follow_links(true)
                .min_depth(1)
                .into_iter()
                .filter_entry(|e| e.file_type().is_file())
                .count(),
            1,
            "Number of objects is wrong"
        );
    }

    #[actix_rt::test]
    async fn download_small_object() {
        test_object_download(16).await
    }

    #[actix_rt::test]
    async fn download_large_object() {
        test_object_download(100_000_000).await
    }
}
