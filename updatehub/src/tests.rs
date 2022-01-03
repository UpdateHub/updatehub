// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::firmware::{
    installation_set::Set,
    tests::{
        create_fake_installation_set, create_fake_starup_callbacks, create_hook,
        device_attributes_dir, device_identity_dir, hardware_hook, product_uid_hook,
        state_change_hook, validate_hook, version_hook,
    },
};
use sdk::api::info::runtime_settings::InstallationSet;
use std::{any::Any, env, fs, io::Write, os::unix::fs::PermissionsExt, path::PathBuf};

pub use crate::{
    firmware::Metadata, runtime_settings::RuntimeSettings, settings::Settings,
    states::machine::Context,
};

pub struct TestEnvironment {
    pub firmware: Data<Metadata>,
    pub runtime_settings: Data<RuntimeSettings>,
    pub settings: Data<Settings>,
    pub binaries: Data<PathBuf>,
}

pub struct Data<T> {
    pub stored_path: PathBuf,
    #[allow(dead_code)]
    pub guard: Vec<Box<dyn Any>>,
    pub data: T,
}

#[derive(Default)]
pub struct TestEnvironmentBuilder {
    disable_polling: bool,
    invalid_hardware: bool,
    extra_binaries: Vec<String>,
    server_address: Option<String>,
    listen_socket: Option<String>,
    supported_install_modes: Option<Vec<&'static str>>,
    state_change_callback: Option<String>,
    validate_callback: Option<String>,
    booting_from_update: bool,
}

impl TestEnvironment {
    pub fn build() -> TestEnvironmentBuilder {
        TestEnvironmentBuilder::default()
    }

    pub fn gen_context(&self) -> Context {
        Context::new(
            self.settings.data.clone(),
            self.runtime_settings.data.clone(),
            self.firmware.data.clone(),
        )
    }
}

impl TestEnvironmentBuilder {
    #[must_use]
    pub fn add_echo_binary(mut self, binary_name: &str) -> Self {
        self.extra_binaries.push(binary_name.to_owned());
        self
    }

    #[must_use]
    pub fn invalid_hardware(self) -> Self {
        TestEnvironmentBuilder { invalid_hardware: true, ..self }
    }

    #[must_use]
    pub fn disable_polling(self) -> Self {
        TestEnvironmentBuilder { disable_polling: true, ..self }
    }

    #[must_use]
    pub fn listen_socket(self, s: String) -> Self {
        TestEnvironmentBuilder { listen_socket: Some(s), ..self }
    }

    #[must_use]
    pub fn server_address(self, s: String) -> Self {
        TestEnvironmentBuilder { server_address: Some(s), ..self }
    }

    #[must_use]
    pub fn supported_install_modes(self, list: Vec<&'static str>) -> Self {
        TestEnvironmentBuilder { supported_install_modes: Some(list), ..self }
    }

    #[must_use]
    pub fn state_change_callback(self, script: String) -> Self {
        TestEnvironmentBuilder { state_change_callback: Some(script), ..self }
    }

    #[must_use]
    pub fn validate_callback(self, script: String) -> Self {
        TestEnvironmentBuilder { validate_callback: Some(script), ..self }
    }

    #[must_use]
    pub fn booting_from_update(self) -> Self {
        TestEnvironmentBuilder { booting_from_update: true, ..self }
    }

    #[allow(clippy::field_reassign_with_default)]
    pub fn finish(self) -> TestEnvironment {
        let firmware = {
            let dir = tempfile::tempdir().unwrap();
            let dir_path = dir.path();

            // create fake hooks to be used to validate the load
            create_hook(
                product_uid_hook(dir_path),
                "#!/bin/sh\necho 229ffd7e08721d716163fc81a2dbaf6c90d449f0a3b009b6a2defe8a0b0d7381",
            );

            create_hook(version_hook(dir_path), "#!/bin/sh\necho 1.1");
            create_hook(
                hardware_hook(dir_path),
                &format!(
                    "#!/bin/sh\necho {}",
                    match self.invalid_hardware {
                        false => "board",
                        true => "invalid",
                    }
                ),
            );
            create_hook(
                device_identity_dir(dir_path),
                "#!/bin/sh\necho id1=value1\necho id2=value2",
            );
            create_hook(
                device_attributes_dir(dir_path),
                "#!/bin/sh\necho attr1=attrvalue1\necho attr2=attrvalue2",
            );

            if let Some(script) = self.state_change_callback {
                create_hook(state_change_hook(dir_path), &script);
            }

            Data {
                data: Metadata::from_path(dir_path).unwrap(),
                stored_path: dir_path.to_owned(),
                guard: vec![Box::new(dir)],
            }
        };

        let binaries = {
            let bin_dir = tempfile::tempdir().unwrap();
            let bin_dir_path = bin_dir.path();
            let output_file = bin_dir_path.join("output");

            create_fake_installation_set(bin_dir_path, 0);
            // Startup callbacks will be stored in the firmware directory
            create_fake_starup_callbacks(&firmware.stored_path, &output_file);

            if let Some(script) = self.validate_callback {
                // Overwrite the validate callback
                create_hook(validate_hook(&firmware.stored_path), &script);
            }

            for bin in self.extra_binaries.into_iter() {
                let mut file = fs::File::create(&bin_dir_path.join(&bin)).unwrap();
                writeln!(file, "#!/bin/sh\necho {} $@ >> {}", bin, output_file.to_string_lossy())
                    .unwrap();
                let mut permissions = file.metadata().unwrap().permissions();
                permissions.set_mode(0o755);
                file.set_permissions(permissions).unwrap();
            }
            let curr_path = env::var("PATH").map(|s| ":".to_string() + &s).unwrap_or_default();
            env::set_var("PATH", format!("{}{}", bin_dir_path.to_string_lossy(), curr_path,));

            Data {
                data: output_file,
                stored_path: bin_dir_path.to_owned(),
                guard: vec![Box::new(bin_dir)],
            }
        };

        let runtime_settings = {
            let file = tempfile::NamedTempFile::new().unwrap();
            let file_path = file.path().to_owned();
            fs::remove_file(&file_path).unwrap();

            let mut runtime_settings = RuntimeSettings::default();
            runtime_settings.path = file_path.clone();
            if self.booting_from_update {
                runtime_settings.enable_persistency();
                runtime_settings.set_upgrading_to(Set(InstallationSet::A)).unwrap();
            }

            Data { data: runtime_settings, stored_path: file_path, guard: vec![Box::new(file)] }
        };

        let settings = {
            let mut file = tempfile::NamedTempFile::new().unwrap();
            let file_path = file.path().to_owned();
            let download_dir = tempfile::tempdir().unwrap();
            let install_modes =
                self.supported_install_modes.unwrap_or_else(|| vec!["copy", "tarball", "test"]);

            write!(
                file,
                r#"[network]
server_address={}
listen_socket={}

[storage]
read_only=false
runtime_settings={runtime_settings}

[polling]
enabled={polling_enabled}
interval="1d"

[update]
download_dir={download_dir}
supported_install_modes={install_modes}

[firmware]
metadata={metadata}"#,
                server_address = toml::to_string(
                    self.server_address.as_deref().unwrap_or("https://api.updatehub.io")
                )
                .unwrap(),
                listen_socket =
                    toml::to_string(self.listen_socket.as_deref().unwrap_or("localhost:8080"))
                        .unwrap(),
                runtime_settings = toml::to_string(&runtime_settings.stored_path).unwrap(),
                polling_enabled = toml::to_string(&!self.disable_polling).unwrap(),
                download_dir = toml::to_string(download_dir.path()).unwrap(),
                install_modes = toml::to_string(&install_modes).unwrap(),
                metadata = toml::to_string(&firmware.stored_path).unwrap()
            )
            .unwrap();

            Data {
                data: Settings::load(&file_path).unwrap(),
                stored_path: file_path,
                guard: vec![Box::new(file), Box::new(download_dir)],
            }
        };

        TestEnvironment { firmware, runtime_settings, settings, binaries }
    }
}
