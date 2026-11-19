// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

mod hook;
pub mod installation_set;

#[cfg(any(test, feature = "test-env"))]
pub mod tests;

use self::hook::{run_hook, run_hooks_from_dir};
use derive_more::{Deref, DerefMut, Display, Error, From};
pub use sdk::api::info::firmware as api;
use slog_scope::{error, info};
use std::{io, path::Path};

const PRODUCT_UID_HOOK: &str = "product-uid";
const VERSION_HOOK: &str = "version";
const HARDWARE_HOOK: &str = "hardware";
const PUB_KEY: &str = "key.pub";
const DEVICE_IDENTITY_DIR: &str = "device-identity.d";
const DEVICE_ATTRIBUTES_DIR: &str = "device-attributes.d";
const STATE_CHANGE_CALLBACK: &str = "state-change-callback";
const VALIDATE_CALLBACK: &str = "validate-callback";
const ROLLBACK_CALLBACK: &str = "rollback-callback";

pub type Result<T> = std::result::Result<T, Error>;

#[derive(Debug, Display, Error, From)]
pub enum Error {
    #[display("invalid product UID")]
    InvalidProductUid,

    #[display("product UID is missing")]
    MissingProductUid,

    #[display("device identity is missing")]
    MissingDeviceIdentity,

    #[display("invalid installation set, the only know ones are 0 or 1")]
    InvalidInstallSet,

    #[display(
        "refusing to run {_0:?} from a test build: tests reboot through a fake `reboot` in the \
         temporary directory, so this one would reboot the machine running them"
    )]
    #[from(ignore)]
    RealRebootInTestBuild(#[error(not(source))] Option<std::path::PathBuf>),

    Walkdir(walkdir::Error),

    Io(std::io::Error),

    Process(easy_process::Error),
}

#[derive(Debug, PartialEq, Eq)]
pub(crate) enum Transition {
    Continue,
    Cancel,
}

#[derive(Clone, Debug, Deref, DerefMut, PartialEq, Eq)]
pub struct Metadata(pub api::Metadata);

impl Metadata {
    /// Collects the firmware metadata by running the hooks stored under `path`.
    ///
    /// # Errors
    ///
    /// Returns an error when a hook is missing, when a hook fails, or when the
    /// output of a hook does not have the expected form.
    pub fn from_path(path: &Path) -> Result<Self> {
        let product_uid_hook = path.join(PRODUCT_UID_HOOK);
        let version_hook = path.join(VERSION_HOOK);
        let hardware_hook = path.join(HARDWARE_HOOK);
        let device_identity_dir = path.join(DEVICE_IDENTITY_DIR);
        let device_attributes_dir = path.join(DEVICE_ATTRIBUTES_DIR);
        let pub_key_path = path.join(PUB_KEY);

        let metadata = Metadata(api::Metadata {
            product_uid: run_hook(&product_uid_hook)?,
            version: run_hook(&version_hook)?,
            hardware: run_hook(&hardware_hook)?,
            pub_key: if pub_key_path.exists() { Some(pub_key_path) } else { None },
            device_identity: run_hooks_from_dir(&device_identity_dir)?,
            device_attributes: run_hooks_from_dir(&device_attributes_dir).unwrap_or_default(),
        });

        if metadata.product_uid.is_empty() {
            return Err(Error::MissingProductUid);
        }

        if metadata.product_uid.len() != 64 {
            return Err(Error::InvalidProductUid);
        }

        if metadata.device_identity.is_empty() {
            return Err(Error::MissingDeviceIdentity);
        }

        Ok(metadata)
    }

    pub(crate) fn as_cloud_metadata(&self) -> cloud::api::FirmwareMetadata<'_> {
        cloud::api::FirmwareMetadata {
            product_uid: &self.0.product_uid,
            version: &self.0.version,
            hardware: &self.0.hardware,
            device_identity: cloud::api::MetadataValue(&self.0.device_identity.0),
            device_attributes: cloud::api::MetadataValue(&self.0.device_attributes.0),
        }
    }
}

pub(crate) fn state_change_callback(path: &Path, state: &str) -> Result<Transition> {
    let callback = path.join(STATE_CHANGE_CALLBACK);
    if !callback.exists() {
        return Ok(Transition::Continue);
    }

    info!("running state change callback for '{}' state", state);

    let output = run_command_for_state(
        &format!("{state} callback"),
        &format!("{} {}", &callback.to_string_lossy(), &state),
    )?;

    match output.stdout.trim() {
        "cancel" => Ok(Transition::Cancel),
        "" => Ok(Transition::Continue),
        _ => Err(io::Error::new(
            io::ErrorKind::InvalidInput,
            format!(
                "invalid output format from 'state-change-callback' hook for state '{}'",
                &state
            ),
        )
        .into()),
    }
}

pub(crate) fn reboot() -> Result<easy_process::Output> {
    #[cfg(any(test, feature = "test-env"))]
    let reboot = &fake_reboot()?.display().to_string();
    #[cfg(not(any(test, feature = "test-env")))]
    let reboot = "reboot";

    Ok(easy_process::run(reboot)?)
}

/// Test builds (unit tests and the `test-env` feature the integration tests
/// use) run the real agent on the developer's machine, with a fake `reboot`
/// first in `PATH`. If that fake is ever gone, e.g. an agent outliving the
/// test that created it, `PATH` would find the real one and reboot the
/// machine. So these builds only run a `reboot` living in the temporary
/// directory, where every test creates its fakes, and run it by its path so
/// no other lookup can pick another.
#[cfg(any(test, feature = "test-env"))]
fn fake_reboot() -> Result<std::path::PathBuf> {
    use std::os::unix::fs::PermissionsExt;

    let found = std::env::var_os("PATH")
        .map(|path| std::env::split_paths(&path).map(|dir| dir.join("reboot")).collect::<Vec<_>>())
        .unwrap_or_default()
        .into_iter()
        .find(|candidate| {
            std::fs::metadata(candidate)
                .is_ok_and(|m| m.is_file() && m.permissions().mode() & 0o111 != 0)
        })
        .map(|reboot| std::fs::canonicalize(&reboot).ok());
    let temp = std::fs::canonicalize(std::env::temp_dir()).ok();
    // The path is run as a command line, which must not split it
    let plain = |p: &std::path::Path| {
        !p.to_string_lossy().contains(|c: char| c.is_whitespace() || "'\"\\".contains(c))
    };

    match (found.flatten(), temp) {
        (Some(reboot), Some(temp)) if reboot.starts_with(&temp) && plain(&reboot) => Ok(reboot),
        (other, _) => {
            error!("refusing to run {:?}, which is not a fake reboot from a test", other);
            Err(Error::RealRebootInTestBuild(other))
        }
    }
}

pub(crate) fn validate_callback(path: &Path) -> Result<Transition> {
    let callback = path.join(VALIDATE_CALLBACK);
    if !callback.exists() {
        return Ok(Transition::Continue);
    }

    info!("running validate callback");

    match run_command_for_state("validate callback", &callback.to_string_lossy()) {
        // We continue the transition in case the validation callback executes fine.
        Ok(_) => Ok(Transition::Continue),

        // In the case of the validation callback exits with error, we cancel
        // the transition so we can do a rollback of the update.
        Err(Error::Process(_)) => Ok(Transition::Cancel),

        // FIXME: We likely need to return Transition::Cancel here but we need
        // to check what are the possible error cases and verify if we cannot
        // handle some of them more gracefully.
        Err(e) => Err(e),
    }
}

pub(crate) fn rollback_callback(path: &Path) -> Result<()> {
    let callback = path.join(ROLLBACK_CALLBACK);
    if !callback.exists() {
        return Ok(());
    }

    info!("running rollback callback");

    run_command_for_state("rollback callback", &callback.to_string_lossy())?;

    Ok(())
}

fn run_command_for_state(name: &str, cmd: &str) -> Result<easy_process::Output> {
    match easy_process::run(cmd) {
        Ok(output) => {
            info!("{} has exit with success", name);
            for err in output.stderr.lines() {
                error!("{} (stderr): {}", name, err);
            }

            Ok(output)
        }
        Err(easy_process::Error::Failure(status, output)) => {
            error!("{} has failed with status: {}", name, status);
            for err in output.stderr.lines() {
                error!("{} (stderr): {}", name, err);
            }
            Err(easy_process::Error::Failure(status, output).into())
        }
        Err(e) => {
            error!("{} has failed with an invalid error: {:?}", name, e);
            Err(e.into())
        }
    }
}

#[cfg(test)]
mod reboot_guard {
    use super::*;
    use crate::utils::test_env::PathEnvGuard;
    use std::{fs, os::unix::fs::PermissionsExt};

    /// A `reboot` that only leaves a mark, so a broken guard fails the test
    /// instead of rebooting anything
    fn fake_reboot(dir: &Path) -> std::path::PathBuf {
        let mark = dir.join("rebooted");
        let reboot = dir.join("reboot");
        fs::write(&reboot, format!("#!/bin/sh\n: > {}\n", mark.display())).unwrap();
        fs::set_permissions(&reboot, fs::Permissions::from_mode(0o755)).unwrap();
        mark
    }

    #[test]
    fn runs_a_fake_reboot_from_the_temporary_directory() {
        let dir = tempfile::tempdir().unwrap();
        let mark = fake_reboot(dir.path());
        let _path = PathEnvGuard::set(dir.path());

        reboot().unwrap();
        assert!(mark.exists());
    }

    #[test]
    fn refuses_a_reboot_outside_the_temporary_directory() {
        // Stands for the system's reboot, which also lives outside it
        let outside = Path::new(env!("CARGO_MANIFEST_DIR"));
        if outside.canonicalize().unwrap().starts_with(std::env::temp_dir().canonicalize().unwrap())
        {
            eprintln!("skipped: the source tree is inside the temporary directory");
            return;
        }
        let dir = tempfile::tempdir_in(outside).unwrap();
        let mark = fake_reboot(dir.path());
        let _path = PathEnvGuard::set(dir.path());

        assert!(matches!(reboot(), Err(Error::RealRebootInTestBuild(Some(_)))));
        assert!(!mark.exists());
    }

    #[test]
    fn ignores_a_reboot_that_is_not_executable() {
        let dir = tempfile::tempdir().unwrap();
        let mark = fake_reboot(dir.path());
        fs::set_permissions(dir.path().join("reboot"), fs::Permissions::from_mode(0o644)).unwrap();
        let _path = PathEnvGuard::set(dir.path());

        assert!(matches!(reboot(), Err(Error::RealRebootInTestBuild(None))));
        assert!(!mark.exists());
    }

    #[test]
    fn refuses_when_there_is_no_fake_reboot() {
        let dir = tempfile::tempdir().unwrap();
        let _path = PathEnvGuard::set(dir.path());

        assert!(matches!(reboot(), Err(Error::RealRebootInTestBuild(None))));
    }
}
