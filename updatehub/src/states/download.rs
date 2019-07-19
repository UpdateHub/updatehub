// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{
    actor::download_abort, Idle, Install, ProgressReporter, State, StateChangeImpl, StateMachine,
    TransitionCallback,
};
use crate::{
    client::Api,
    firmware::installation_set,
    object::{self, Info},
    update_package::UpdatePackage,
};
use failure::bail;

#[derive(Debug, PartialEq)]
pub(super) struct Download {
    pub(super) update_package: UpdatePackage,
    pub(super) installation_set: installation_set::Set,
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

impl StateChangeImpl for State<Download> {
    fn name(&self) -> &'static str {
        "download"
    }

    fn handle_download_abort(&self) -> download_abort::Response {
        download_abort::Response::RequestAccepted
    }

    fn handle(self) -> Result<StateMachine, failure::Error> {
        let installation_set = self.0.installation_set;
        let download_dir = &shared_state!().settings.update.download_dir.clone();

        // Download the missing or incomplete objects
        for object in self
            .0
            .update_package
            .filter_objects(
                &shared_state!().settings,
                installation_set,
                object::info::Status::Missing,
            )
            .into_iter()
            .chain(self.0.update_package.filter_objects(
                &shared_state!().settings,
                installation_set,
                object::info::Status::Incomplete,
            ))
        {
            Api::new(&shared_state!().settings.network.server_address).download_object(
                &shared_state!().firmware.product_uid,
                &self.0.update_package.package_uid(),
                download_dir,
                object.sha256sum(),
            )?;
        }

        if self
            .0
            .update_package
            .objects(installation_set)
            .iter()
            .all(|o| o.status(download_dir).ok() == Some(object::info::Status::Ready))
        {
            Ok(StateMachine::Install(self.into()))
        } else {
            bail!("Not all objects are ready for use")
        }
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use crate::states::PrepareDownload;
    use pretty_assertions::assert_eq;
    use walkdir::WalkDir;

    fn fake_download_state() -> State<PrepareDownload> {
        use crate::{
            firmware::{
                tests::{create_fake_installation_set, create_fake_metadata, FakeDevice},
                Metadata,
            },
            runtime_settings::RuntimeSettings,
            update_package::tests::{create_fake_settings, get_update_package},
        };
        use std::{env, fs::create_dir_all};

        let settings = create_fake_settings();
        let tmpdir = settings.update.download_dir.clone();
        let _ = create_dir_all(&tmpdir);
        create_fake_installation_set(&tmpdir, 0);
        env::set_var("PATH", format!("{}", &tmpdir.to_string_lossy()));
        let runtime_settings = RuntimeSettings::default();
        let firmware = Metadata::from_path(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap();
        set_shared_state!(settings, runtime_settings, firmware);

        State(PrepareDownload {
            update_package: get_update_package(),
        })
    }

    #[test]
    fn skip_download_if_ready() {
        use crate::update_package::tests::create_fake_object;

        let predownload_state = fake_download_state();
        let tmpdir = shared_state!().settings.update.download_dir.clone();

        create_fake_object(&shared_state!().settings);

        let machine = StateMachine::PrepareDownload(predownload_state).move_to_next_state();
        assert_state!(machine, Download);
        let machine = machine.unwrap().move_to_next_state();
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

    #[test]
    fn download_objects() {
        use crypto_hash::{hex_digest, Algorithm};
        use mockito::mock;
        use std::{fs::File, io::Read};

        let sha256sum = "c775e7b757ede630cd0aa1113bd102661ab38829ca52a6422ab782862f268646";
        let predownload_state = fake_download_state();
        let tmpdir = shared_state!().settings.update.download_dir.clone();

        // leftover file to ensure it is removed
        let _ = File::create(&tmpdir.join("leftover-file"));

        let mock = mock(
            "GET",
            format!(
                "/products/{}/packages/{}/objects/{}",
                "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
                &predownload_state.0.update_package.package_uid(),
                &sha256sum
            )
            .as_str(),
        )
        .match_header("Content-Type", "application/json")
        .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
        .with_status(200)
        .with_body("1234567890")
        .create();

        let machine = StateMachine::PrepareDownload(predownload_state).move_to_next_state();
        assert_state!(machine, Download);
        let machine = machine.unwrap().move_to_next_state();
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
        let _ = File::open(&tmpdir.join(&sha256sum))
            .expect("Fail to open the temporary directory.")
            .read_to_string(&mut object_content);

        assert_eq!(
            &hex_digest(Algorithm::SHA256, object_content.as_bytes()),
            &sha256sum,
            "Checksum mismatch"
        );
    }
}
