// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Context, Error, Result};
use crate::{
    object::{Info, Installer},
    utils::{self, definitions::TargetTypeExt},
};

use pkg_schema::{definitions, objects};
use slog_scope::info;

#[async_trait::async_trait(?Send)]
impl Installer for objects::Flash {
    async fn check_requirements(&self, _: &Context) -> Result<()> {
        info!("'flash' handle checking requirements");
        utils::fs::is_executable_in_path("nandwrite")?;
        utils::fs::is_executable_in_path("flashcp")?;
        utils::fs::is_executable_in_path("flash_erase")?;

        match self.target {
            definitions::TargetType::Device(_) | definitions::TargetType::MTDName(_) => {
                self.target.valid()?;
                utils::fs::ensure_disk_space(
                    &self.target.get_target()?,
                    self.required_install_size(),
                )?;
                Ok(())
            }
            _ => Err(Error::InvalidTargetType(self.target.clone())),
        }
    }

    async fn install(&self, context: &Context) -> Result<()> {
        info!("'flash' handler Install {} ({})", self.filename, self.sha256sum);

        let target = self.target.get_target()?;
        let source = context.download_dir.join(self.sha256sum());

        if super::should_skip_install(&self.install_if_different, &self.sha256sum, async {
            tokio::fs::File::open(&target).await.map_err(Error::from)
        })
        .await?
        {
            return Ok(());
        }

        let is_nand = utils::mtd::is_nand(&target)?;

        easy_process::run(&format!("flash_erase {:?} 0 0", target))?;

        if is_nand {
            easy_process::run(&format!("nandwrite -p {:?} {:?}", target, source))?;
        } else {
            easy_process::run(&format!("flashcp {:?} {:?}", source, target))?;
        }

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{
        object::installer::tests::create_echo_bins,
        utils::mtd::tests::{FakeMtd, MtdKind, SERIALIZE},
    };
    use pretty_assertions::assert_eq;
    use std::env;

    fn fake_flash_obj(target: &str) -> objects::Flash {
        objects::Flash {
            filename: "etc/passwd".to_string(),
            size: 1024,
            sha256sum: "cfe2be1c64b03875008".to_string(),
            target: definitions::TargetType::MTDName(target.to_string()),

            install_if_different: None,
        }
    }

    #[tokio::test]
    async fn check_requirements_with_missing_binaries() {
        let flash_obj = fake_flash_obj("system0");
        let context = Context::default();

        env::set_var("PATH", "");
        let (_handle, _) = create_echo_bins(&["flash_erase"]).unwrap();
        assert!(flash_obj.check_requirements(&context).await.is_err());

        env::set_var("PATH", "");
        let (_handle, _) = create_echo_bins(&["flashcp"]).unwrap();
        assert!(flash_obj.check_requirements(&context).await.is_err());

        env::set_var("PATH", "");
        let (_handle, _) = create_echo_bins(&["nandwrite"]).unwrap();
        assert!(flash_obj.check_requirements(&context).await.is_err());

        env::set_var("PATH", "");
        let (_handle, _) = create_echo_bins(&["flash_erase", "nandwrite"]).unwrap();
        assert!(flash_obj.check_requirements(&context).await.is_err());

        env::set_var("PATH", "");
        let (_handle, _) = create_echo_bins(&["flash_erase", "flashcp"]).unwrap();
        assert!(flash_obj.check_requirements(&context).await.is_err());

        env::set_var("PATH", "");
        let (_handle, _) = create_echo_bins(&["nandwrite", "nandwrite"]).unwrap();
        assert!(flash_obj.check_requirements(&context).await.is_err());
    }

    #[tokio::test]
    #[ignore]
    async fn install_nor() {
        let _mtd_lock = SERIALIZE.lock();
        let mtd = FakeMtd::new(&["system0"], MtdKind::Nor).unwrap();
        let target = &mtd.devices[0];
        let flash_obj = fake_flash_obj("system0");
        let download_dir = tempfile::tempdir().unwrap();
        let source = download_dir.path().join(&flash_obj.sha256sum);
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };

        let (_handle, calls) = create_echo_bins(&["flash_erase", "flashcp", "nandwrite"]).unwrap();

        flash_obj.check_requirements(&context).await.unwrap();
        flash_obj.install(&context).await.unwrap();

        let expected = format!(
            "flash_erase {} 0 0\nflashcp {} {}\n",
            target.to_str().unwrap(),
            source.to_str().unwrap(),
            target.to_str().unwrap()
        );

        assert_eq!(std::fs::read_to_string(calls).unwrap(), expected);
    }
}
