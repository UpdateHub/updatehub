// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

mod copy;
mod flash;
mod imxkobs;
mod raw;
mod tarball;
mod test;
mod ubifs;

use super::{Error, Result};
use pkg_schema::Object;
use slog_scope::debug;

pub(crate) trait Installer {
    fn check_requirements(&self) -> Result<()> {
        debug!("running default check_requirements");
        Ok(())
    }

    fn setup(&mut self) -> Result<()> {
        debug!("running default setup");
        Ok(())
    }

    fn cleanup(&mut self) -> Result<()> {
        debug!("running default cleanup");
        Ok(())
    }

    fn install(&self, download_dir: &std::path::Path) -> Result<()>;
}

impl Installer for Object {
    fn check_requirements(&self) -> Result<()> {
        for_any_object!(self, o, { o.check_requirements() })
    }

    fn setup(&mut self) -> Result<()> {
        for_any_object!(self, o, { o.setup() })
    }

    fn install(&self, download_dir: &std::path::Path) -> Result<()> {
        for_any_object!(self, o, { o.install(download_dir) })
    }

    fn cleanup(&mut self) -> Result<()> {
        for_any_object!(self, o, { o.cleanup() })
    }
}

#[cfg(test)]
mod tests {
    use lazy_static::lazy_static;
    use std::{
        env, fs,
        io::Write,
        os::unix::fs::PermissionsExt,
        path::{Path, PathBuf},
        sync::{Arc, Mutex},
    };
    use tempfile::TempDir;

    // Used to serialize access to Loop devices across tests
    lazy_static! {
        pub static ref SERIALIZE: Arc<Mutex<()>> = Arc::new(Mutex::default());
    }

    fn create_echo_bin(bin: &Path, output: &Path) -> std::io::Result<()> {
        let mut file = std::fs::File::create(bin)?;
        file.write_all(
            format!(
                "#!/bin/sh\necho {} $@ >> {:?}\n",
                bin.file_name().unwrap().to_str().unwrap(),
                output
            )
            .as_bytes(),
        )?;
        file.set_permissions(fs::Permissions::from_mode(0o777))?;

        Ok(())
    }

    pub fn create_echo_bins(bins: &[&str]) -> std::io::Result<(TempDir, PathBuf)> {
        let mocks = tempfile::tempdir()?;
        let mocks_dir = mocks.path();
        let calls = mocks_dir.join("calls");

        for bin in bins {
            create_echo_bin(&mocks_dir.join(bin), &calls)?;
        }

        env::set_var(
            "PATH",
            format!(
                "{}{}",
                mocks_dir.display(),
                &env::var("PATH").map(|s| format!(":{}", s)).unwrap_or_default()
            ),
        );

        Ok((mocks, calls))
    }
}
