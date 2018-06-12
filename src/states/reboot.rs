// Copyright (C) 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: MPL-2.0
//

use easy_process;
use failure::Error;
use states::{Idle, State, StateChangeImpl, StateMachine};

#[derive(Debug, PartialEq)]
pub struct Reboot {}

create_state_step!(Reboot => Idle);

impl StateChangeImpl for State<Reboot> {
    // FIXME: When adding state-chance hooks, we need to go to Idle if
    // cancelled.
    fn handle(self) -> Result<StateMachine, Error> {
        info!("Triggering reboot");
        let output = easy_process::run("reboot")?;
        if !output.stdout.is_empty() || !output.stderr.is_empty() {
            info!(
                "  reboot output: stdout: {}, stderr: {}",
                output.stdout, output.stderr
            );
        }
        Ok(StateMachine::Idle(self.into()))
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use mktemp::Temp;
    use std::path::Path;

    fn create_reboot(path: &Path) {
        use std::fs::create_dir_all;
        use std::fs::metadata;
        use std::fs::File;
        use std::io::Write;
        use std::os::unix::fs::PermissionsExt;

        // ensure path exists
        create_dir_all(path).unwrap();

        let mut file = File::create(&path.join("reboot")).unwrap();
        file.write_all(b"#!/bin/sh\necho reboot").unwrap();

        let mut permissions = metadata(path).unwrap().permissions();
        permissions.set_mode(0o755);
        file.set_permissions(permissions).unwrap();
    }

    #[test]
    fn runs() {
        use firmware::tests::{create_fake_metadata, FakeDevice};
        use firmware::Metadata;
        use runtime_settings::RuntimeSettings;
        use settings::Settings;
        use std::env;

        // create the fake reboot command
        let tmpdir = Temp::new_dir().unwrap().to_path_buf();
        create_reboot(&tmpdir);
        env::set_var(
            "PATH",
            format!(
                "{}:{}",
                tmpdir.as_path().to_string_lossy(),
                env::var("PATH").unwrap_or_default()
            ),
        );

        let machine = StateMachine::Reboot(State {
            settings: Settings::default(),
            runtime_settings: RuntimeSettings::default(),
            firmware: Metadata::new(&create_fake_metadata(FakeDevice::NoUpdate)).unwrap(),
            applied_package_uid: None,
            state: Reboot {},
        }).move_to_next_state();

        assert!(machine.is_ok(), "Error: {:?}", machine);
        assert_state!(machine, Idle);
    }
}
