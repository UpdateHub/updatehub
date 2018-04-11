// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use client::Api;
use failure::Error;
use states::idle::Idle;
use states::install::Install;
use states::{State, StateChangeImpl, StateMachine};
use std::fs::remove_file;
use update_package::{ObjectStatus, UpdatePackage};
use walkdir::WalkDir;

#[derive(Debug, PartialEq)]
pub struct Download {
    pub update_package: UpdatePackage,
}

create_state_step!(Download => Idle);
create_state_step!(Download => Install(update_package));

impl StateChangeImpl for State<Download> {
    fn to_next_state(self) -> Result<StateMachine, Error> {
        // Prune left over from previous installations
        WalkDir::new(&self.settings.update.download_dir)
            .follow_links(true)
            .min_depth(1)
            .into_iter()
            .filter_entry(|e| e.file_type().is_file())
            .filter_map(|e| e.ok())
            .filter(|e| {
                !self.state
                    .update_package
                    .objects()
                    .iter()
                    .map(|o| o.sha256sum())
                    .collect::<Vec<_>>()
                    .contains(&e.file_name().to_str().unwrap_or(""))
            })
            .for_each(|e| {
                remove_file(e.path()).unwrap_or_else(|err| {
                    error!("Fail to remove file: {} (err: {})", e.path().display(), err)
                })
            });

        // Prune corrupted files
        self.state
            .update_package
            .filter_objects(&self.settings, ObjectStatus::Corrupted)
            .into_iter()
            .for_each(|o| {
                remove_file(&self.settings.update.download_dir.join(o.sha256sum())).unwrap_or_else(
                    |err| error!("Fail to remove file: {} (err: {})", o.sha256sum(), err),
                )
            });

        // Download the missing or incomplete objects
        self.state
            .update_package
            .filter_objects(&self.settings, ObjectStatus::Missing)
            .into_iter()
            .chain(
                self.state
                    .update_package
                    .filter_objects(&self.settings, ObjectStatus::Incomplete),
            )
            .for_each(|o| {
                Api::new(&self.settings, &self.runtime_settings, &self.firmware)
                    .download_object(
                        &self.state.update_package.package_uid().unwrap(),
                        o.sha256sum(),
                    )
                    .unwrap_or_else(|err| {
                        error!("Fail to download object: {} (err: {})", o.sha256sum(), err);
                    });
            });

        // FIXME: Must return error when failing to download
        Ok(StateMachine::Install(self.into()))
    }
}

#[test]
fn skip_download_if_ready() {
    use super::*;
    use firmware::tests::{create_fake_metadata, FakeDevice};
    use std::fs::create_dir_all;
    use update_package::tests::{create_fake_object, create_fake_settings, get_update_package};

    let settings = create_fake_settings();
    let tmpdir = settings.update.download_dir.clone();
    let _ = create_dir_all(&tmpdir);
    let _ = create_fake_object(&settings);

    let machine = StateMachine::Download(State {
        settings: settings,
        runtime_settings: RuntimeSettings::default(),
        firmware: Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
        applied_package_uid: None,
        state: Download {
            update_package: get_update_package(),
        },
    }).step();

    assert_eq!(
        WalkDir::new(&tmpdir)
            .follow_links(true)
            .min_depth(1)
            .into_iter()
            .filter_entry(|e| e.file_type().is_file())
            .filter_map(|e| e.ok())
            .count(),
        1,
        "Number of objects is wrong"
    );

    assert_state!(machine, Install);
}

#[test]
fn download_objects() {
    use super::*;
    use crypto_hash::{hex_digest, Algorithm};
    use firmware::tests::{create_fake_metadata, FakeDevice};
    use mockito::mock;
    use std::fs::create_dir_all;
    use std::fs::File;
    use std::io::Read;
    use update_package::tests::{create_fake_settings, get_update_package};

    let settings = create_fake_settings();
    let update_package = get_update_package();
    let sha256sum = "c775e7b757ede630cd0aa1113bd102661ab38829ca52a6422ab782862f268646";
    let tmpdir = settings.update.download_dir.clone();
    let _ = create_dir_all(&tmpdir);

    // leftover file to ensure it is removed
    let _ = File::create(&tmpdir.join("leftover-file"));

    let mock = mock(
        "GET",
        format!(
            "/products/{}/packages/{}/objects/{}",
            "229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
            &update_package.package_uid().unwrap(),
            &sha256sum
        ).as_str(),
    ).match_header("Content-Type", "application/json")
        .match_header("Api-Content-Type", "application/vnd.updatehub-v1+json")
        .with_status(200)
        .with_body("1234567890")
        .create();

    let machine = StateMachine::Download(State {
        settings: settings,
        runtime_settings: RuntimeSettings::default(),
        firmware: Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
        applied_package_uid: None,
        state: Download {
            update_package: update_package,
        },
    }).step();

    mock.assert();

    assert_eq!(
        WalkDir::new(&tmpdir)
            .follow_links(true)
            .min_depth(1)
            .into_iter()
            .filter_entry(|e| e.file_type().is_file())
            .filter_map(|e| e.ok())
            .count(),
        1,
        "Failed to remove the corrupted object"
    );

    let mut object_content = String::new();
    let _ = File::open(&tmpdir.join(&sha256sum))
        .unwrap()
        .read_to_string(&mut object_content);

    assert_eq!(
        &hex_digest(Algorithm::SHA256, object_content.as_bytes()),
        &sha256sum,
        "Checksum mismatch"
    );

    assert_state!(machine, Install);
}
