// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Error, Result};
use crate::{
    object::{Info, Installer},
    utils::{self, definitions::TargetTypeExt},
};

use pkg_schema::{definitions, objects};
use slog_scope::info;

impl Installer for objects::Ubifs {
    fn check_requirements(&self) -> Result<()> {
        info!("'ubifs' handle checking requirements");
        if self.compressed {
            unimplemented!("FIXME: check the required_uncompressed_size");
        }

        utils::fs::is_executable_in_path("ubiupdatevol")?;
        utils::fs::is_executable_in_path("ubinfo")?;

        if let definitions::TargetType::UBIVolume(_) = self.target.valid()? {
            return Ok(());
        }

        Err(Error::InvalidTargetType(self.target.clone()))
    }

    fn install(&self, download_dir: &std::path::Path) -> Result<()> {
        info!("'ubifs' handler Install");

        let target = self.target.get_target()?;
        let source = download_dir.join(self.sha256sum());

        if self.compressed {
            unimplemented!("FIXME: handle compressed installation");
        } else {
            easy_process::run(&format!("ubiupdatevol {} {}", target.display(), source.display()))?;
        }

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{
        object::installer::tests::create_echo_bins,
        utils::mtd::tests::{FakeUbi, MtdKind, SERIALIZE},
    };
    use pretty_assertions::assert_eq;
    use std::env;

    fn fake_ubifs_obj(name: &str) -> objects::Ubifs {
        objects::Ubifs {
            filename: "ubifs-filename".to_string(),
            size: 1024,
            sha256sum: "e3b0c44298fc1c149afb".to_string(),
            target: definitions::TargetType::UBIVolume(name.to_string()),

            compressed: false,
            required_uncompressed_size: 2048,
        }
    }

    #[test]
    fn check_requirements_with_missing_binaries() {
        let ubifs_obj = fake_ubifs_obj("home");

        env::set_var("PATH", "");
        assert!(ubifs_obj.check_requirements().is_err());

        env::set_var("PATH", "");
        let (_handle, _) = create_echo_bins(&["ubinfo"]).unwrap();
        assert!(ubifs_obj.check_requirements().is_err());

        env::set_var("PATH", "");
        let (_handle, _) = create_echo_bins(&["ubiupdatevol"]).unwrap();
        assert!(ubifs_obj.check_requirements().is_err());
    }

    #[test]
    #[ignore]
    fn install() {
        let _mtd_lock = SERIALIZE.lock();
        let _ubi = FakeUbi::new(&["home"], MtdKind::Nor).unwrap();
        let ubifs_obj = fake_ubifs_obj("home");
        let download_dir = tempfile::tempdir().unwrap();
        let target = ubifs_obj.target.get_target().unwrap();
        let source = download_dir.path().join(&ubifs_obj.sha256sum);

        let (_handle, calls) = create_echo_bins(&["ubiupdatevol"]).unwrap();

        ubifs_obj.check_requirements().unwrap();
        ubifs_obj.install(download_dir.path()).unwrap();

        let expected = format!("ubiupdatevol {} {}\n", target.display(), source.display());
        assert_eq!(std::fs::read_to_string(calls).unwrap(), expected);
    }
}
