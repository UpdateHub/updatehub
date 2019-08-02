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

use pkg_schema::Object;
use slog_scope::debug;

pub(crate) trait Installer {
    fn check_requirements(&self) -> Result<(), failure::Error> {
        debug!("running default check_requirements");
        Ok(())
    }

    fn setup(&mut self) -> Result<(), failure::Error> {
        debug!("running default setup");
        Ok(())
    }

    fn cleanup(&mut self) -> Result<(), failure::Error> {
        debug!("running default cleanup");
        Ok(())
    }

    fn install(&self, download_dir: &std::path::Path) -> Result<(), failure::Error>;
}

impl Installer for Object {
    fn check_requirements(&self) -> Result<(), failure::Error> {
        for_any_object!(self, o, { o.check_requirements() })
    }

    fn setup(&mut self) -> Result<(), failure::Error> {
        for_any_object!(self, o, { o.setup() })
    }

    fn install(&self, download_dir: &std::path::Path) -> Result<(), failure::Error> {
        for_any_object!(self, o, { o.install(download_dir) })
    }

    fn cleanup(&mut self) -> Result<(), failure::Error> {
        for_any_object!(self, o, { o.cleanup() })
    }
}

#[cfg(test)]
mod tests {
    use lazy_static::lazy_static;
    use std::sync::{Arc, Mutex};

    // Used to serialize access to Loop devices across tests
    lazy_static! {
        pub static ref SERIALIZE: Arc<Mutex<()>> = Arc::new(Mutex::default());
    }
}
