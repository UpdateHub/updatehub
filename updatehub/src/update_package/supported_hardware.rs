// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::Error;

pub(crate) use pkg_schema::SupportedHardware;

pub(crate) trait SupportedHardwareExt {
    fn compatible_with(&self, hardware: &str) -> Result<(), Error>;
}

impl SupportedHardwareExt for SupportedHardware {
    fn compatible_with(&self, hardware: &str) -> Result<(), Error> {
        let hardware = hardware.to_string();
        let compatible = match self {
            SupportedHardware::Any => true,
            SupportedHardware::HardwareList(l) => l.contains(&hardware),
        };

        if !compatible {
            return Err(Error::IncompatibleHardware(hardware));
        }

        Ok(())
    }
}
