// Copyright (C) 2021 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Context, Error, Result};
use crate::{
    object::{Info, Installer},
    utils::{self, definitions::TargetTypeExt},
};

use pkg_schema::{definitions, objects};
use slog_scope::info;

impl Installer for objects::Bita {
    fn check_requirements(&self) -> Result<()> {
        info!("'bita' handle checking requirements");
        utils::fs::is_executable_in_path("bita")?;

        // FIXME: the free space we will need to install an object here
        // is not necessarly the size of the bita object, so we might need
        // some support from the bita tool find the right value

        // if let definitions::TargetType::Device(dev) = self.target.valid()? {
        //     utils::fs::ensure_disk_space(&dev, self.required_install_size())?;
        //     return Ok(());
        // }
        // Err(Error::InvalidTargetType(self.target.clone()))

        Ok(())
    }

    fn install(&self, context: &Context) -> Result<()> {
        info!("'bita' handler Install {} ({})", self.filename, self.sha256sum);

        let target = self.target.get_target()?;
        let source = if context.offline_update {
            format!("{:?}", context.download_dir.join(&self.sha256sum))
        } else {
            format!("{}/{}", context.base_url, &self.sha256sum)
        };

        easy_process::run(&format!(
            "bita clone --seed-output {} {}",
            source,
            target.to_string_lossy()
        ))?;

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::object::installer::tests::create_echo_bins;
    use pretty_assertions::assert_eq;
    use std::env;

    fn fake_bita_obj() -> objects::Bita {
        objects::Bita {
            filename: "etc/passwd".to_string(),
            sha256sum: "cfe2be1c64b03875008".to_string(),
            target: definitions::TargetType::Device(std::path::PathBuf::from("/dev/sda1")),
            size: 1024,
        }
    }

    #[test]
    fn check_requirements_with_missing_binary() {
        let bita_obj = fake_bita_obj();

        env::set_var("PATH", "");
        assert!(bita_obj.check_requirements().is_err());
    }

    #[test]
    #[ignore]
    fn install_commands() {
        let bita_obj = fake_bita_obj();
        let download_dir = tempfile::tempdir().unwrap();

        let (_handle, calls) = create_echo_bins(&["bita"]).unwrap();

        let obj_context = Context {
            download_dir: download_dir.path().to_owned(),
            offline_update: false,
            base_url: String::from("https://foo.bar/bita_archive"),
        };
        bita_obj.check_requirements().unwrap();
        bita_obj.install(&obj_context).unwrap();

        let expected = String::from(
            "bita clone --seed-output https://foo.bar/bita_archive/cfe2be1c64b03875008 /dev/sda1\n",
        );

        assert_eq!(std::fs::read_to_string(calls).unwrap(), expected);
    }
}
