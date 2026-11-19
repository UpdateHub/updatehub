// Copyright (C) 2026 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

//! A/B firmware in the eMMC boot partitions, end to end, without hardware.
//!
//! The agent binary runs against a simulated eMMC (`UPDATEHUB_EMMC_SIMULATOR`):
//! the boot partitions are files, EXT_CSD is a file, and `force_ro` is
//! enforced. The test plays everything around the agent: the SoC ROM
//! booting the boot partition PARTITION_CONFIG enables, U-Boot keeping the
//! installation set (and rolling it back on its own), power cycles, and the
//! update server.
//!
//! The scenarios run in order on one device, as its history: each starts
//! from the state the previous one left.

use mockito::{Mock, ServerGuard};
use serde_json::json;
use std::{
    fs,
    io::Write,
    net::TcpListener,
    os::unix::{
        fs::PermissionsExt,
        process::{CommandExt, ExitStatusExt},
    },
    path::{Path, PathBuf},
    process::{Child, Command, ExitStatus, Stdio},
    thread,
    time::{Duration, Instant},
};

const BOOT_SIZE: usize = 1024 * 1024;
const FIP_OFFSET: usize = 256 * 1024;
const CHUNK_SIZE: usize = 128 * 1024;
const FSBL_PAYLOAD: usize = 96 * 1024;
const FIP_PAYLOAD: usize = 320 * 1024;
const PART_CONFIG: usize = 179;
const HEADER: usize = 32;

// ---- firmware, as TF-A + FIP on the STM32MP135 -----------------------------

/// A firmware image: magic, kind, version, length, checksum, payload. The
/// ROM refuses an image whose checksum does not match, as the STM32 one does.
fn image(kind: &[u8; 4], version: &str, len: usize) -> Vec<u8> {
    let seed = version.bytes().chain(kind.iter().copied()).fold(7u8, |a, b| a.wrapping_mul(31) ^ b);
    let payload: Vec<u8> =
        (0..len).map(|i| (i as u8).wrapping_mul(13).wrapping_add(seed) ^ (i >> 8) as u8).collect();
    let mut data = b"UHFW".to_vec();
    data.extend_from_slice(kind);
    let mut v = version.as_bytes().to_vec();
    v.resize(16, 0);
    data.extend_from_slice(&v);
    data.extend_from_slice(&(len as u32).to_le_bytes());
    data.extend_from_slice(&checksum(&payload).to_le_bytes());
    data.extend_from_slice(&payload);
    data
}

fn checksum(data: &[u8]) -> u32 {
    data.iter().fold(0u32, |sum, b| sum.wrapping_add(u32::from(*b)))
}

/// (kind, version) of a complete image at the start of `data`
fn parse(data: &[u8]) -> Option<([u8; 4], String)> {
    if data.len() < HEADER || &data[..4] != b"UHFW" {
        return None;
    }
    let kind = data[4..8].try_into().unwrap();
    let version = String::from_utf8_lossy(&data[8..24]).trim_end_matches('\0').to_owned();
    let len = u32::from_le_bytes(data[24..28].try_into().unwrap()) as usize;
    let sum = u32::from_le_bytes(data[28..32].try_into().unwrap());
    let payload = data.get(HEADER..HEADER + len)?;
    (checksum(payload) == sum).then_some((kind, version))
}

struct Firmware {
    fsbl: Vec<u8>,
    fip: Vec<u8>,
}

impl Firmware {
    fn new(version: &str) -> Self {
        Firmware {
            fsbl: image(b"FSBL", version, FSBL_PAYLOAD),
            fip: image(b"FIP\0", version, FIP_PAYLOAD),
        }
    }
}

// ---- the device: eMMC, ROM and bootloader ---------------------------------

struct Device {
    _dir: tempfile::TempDir,
    /// The eMMC base device; in the simulator, its EXT_CSD
    emmc: PathBuf,
    /// Simulated sysfs, `force_ro` of each boot partition
    sysfs: PathBuf,
    /// The bootloader environment: the active installation set
    active: PathBuf,
    /// Makes the next validation of an update fail
    fail_validation: PathBuf,
    /// Where the system half of an update goes
    rootfs: PathBuf,
}

impl Device {
    /// A device as the installer leaves it: both boot partitions hold the
    /// same firmware, boot0 is enabled, installation set 0 is active.
    fn factory(firmware: &Firmware) -> Self {
        let dir = tempfile::tempdir().unwrap();
        let emmc = dir.path().join("mmcblk0");
        let sysfs = dir.path().join("sys");

        let mut ext_csd = vec![0u8; 512];
        ext_csd[PART_CONFIG] = 0x48; // boot0, boot acknowledge
        fs::write(&emmc, ext_csd).unwrap();

        let device = Device {
            emmc,
            sysfs,
            active: dir.path().join("active"),
            fail_validation: dir.path().join("fail-validation"),
            rootfs: dir.path().join("rootfs"),
            _dir: dir,
        };
        fs::write(&device.rootfs, vec![0u8; CHUNK_SIZE]).unwrap();
        for n in 0..2 {
            let mut data = vec![0u8; BOOT_SIZE];
            data[..firmware.fsbl.len()].copy_from_slice(&firmware.fsbl);
            data[FIP_OFFSET..FIP_OFFSET + firmware.fip.len()].copy_from_slice(&firmware.fip);
            fs::write(device.boot_partition(n), data).unwrap();
            let sys = device.sysfs.join(format!("mmcblk0boot{n}"));
            fs::create_dir_all(&sys).unwrap();
            fs::write(sys.join("force_ro"), "1\n").unwrap();
        }
        device.set_active(0);
        device
    }

    fn lock(&self) {
        for n in 0..2 {
            fs::write(self.sysfs.join(format!("mmcblk0boot{n}/force_ro")), "1\n").unwrap();
        }
    }

    fn boot_partition(&self, n: u8) -> PathBuf {
        PathBuf::from(format!("{}boot{n}", self.emmc.display()))
    }

    fn part_config(&self) -> u8 {
        fs::read(&self.emmc).unwrap()[PART_CONFIG]
    }

    fn set_part_config(&self, value: u8) {
        let mut ext_csd = fs::read(&self.emmc).unwrap();
        ext_csd[PART_CONFIG] = value;
        fs::write(&self.emmc, ext_csd).unwrap();
    }

    fn enabled(&self) -> Option<u8> {
        match (self.part_config() >> 3) & 7 {
            1 => Some(0),
            2 => Some(1),
            _ => None,
        }
    }

    fn force_ro(&self, n: u8) -> String {
        fs::read_to_string(self.sysfs.join(format!("mmcblk0boot{n}/force_ro")))
            .unwrap()
            .trim()
            .to_owned()
    }

    /// Version of the complete firmware in boot partition `n`
    fn firmware(&self, n: u8) -> Option<String> {
        let data = fs::read(self.boot_partition(n)).unwrap();
        let (fsbl_kind, fsbl) = parse(&data)?;
        let (fip_kind, fip) = parse(&data[FIP_OFFSET..])?;
        (&fsbl_kind == b"FSBL" && &fip_kind == b"FIP\0" && fsbl == fip).then_some(fsbl)
    }

    fn active(&self) -> u8 {
        fs::read_to_string(&self.active).unwrap().trim().parse().unwrap()
    }

    /// What U-Boot does when it rolls back, without the agent knowing
    fn set_active(&self, set: u8) {
        fs::write(&self.active, format!("{set}\n")).unwrap();
    }

    /// What the ROM boots: (boot partition, firmware version). A device
    /// whose enabled boot partition holds no complete firmware is bricked.
    fn rom(&self) -> (u8, String) {
        let part = self.enabled().unwrap_or_else(|| {
            panic!("ROM: PARTITION_CONFIG 0x{:02x} enables no boot partition", self.part_config())
        });
        let version = self
            .firmware(part)
            .unwrap_or_else(|| panic!("ROM: boot{part} has no complete firmware, bricked"));
        (part, version)
    }
}

// ---- the agent --------------------------------------------------------------

struct Agent {
    child: Child,
}

struct Harness {
    setup: updatehub::tests::TestEnvironment,
    device: Device,
    server: ServerGuard,
    mocks: Vec<Mock>,
    listen: String,
    log: PathBuf,
    agent: Option<Agent>,
    /// Power-ons so far, which name the kernel's boot ID
    boots: usize,
    runtime: tokio::runtime::Runtime,
}

impl Harness {
    fn new() -> Self {
        let device = Device::factory(&Firmware::new("1.0"));
        let server = mockito::Server::new();
        // The agent listens on IPv4 whatever `localhost` resolves to
        let port = (8080..65535).find(|p| TcpListener::bind(("127.0.0.1", *p)).is_ok()).unwrap();
        let listen = format!("127.0.0.1:{port}");

        let setup = updatehub::tests::TestEnvironment::build()
            .listen_socket(listen.clone())
            .server_address(server.url())
            .disable_polling()
            .supported_install_modes(vec!["raw"])
            .add_echo_binary("reboot")
            .finish();

        // The bootloader environment holds the installation set
        let bin = &setup.binaries.stored_path;
        script(&bin.join("updatehub-active-get"), &format!("cat {}", device.active.display()));
        script(
            &bin.join("updatehub-active-set"),
            &format!("echo $1 > {}", device.active.display()),
        );
        script(
            &setup.firmware.stored_path.join("validate-callback"),
            &format!("test ! -e {}", device.fail_validation.display()),
        );

        let mut settings =
            fs::OpenOptions::new().append(true).open(&setup.settings.stored_path).unwrap();
        write!(settings, "\n\n[firmware.boot-emmc]\ndevice=\"{}\"\n", device.emmc.display())
            .unwrap();

        let log = setup.binaries.stored_path.join("agent.log");
        let mut harness = Harness {
            setup,
            device,
            server,
            mocks: Vec::new(),
            listen,
            log,
            agent: None,
            boots: 0,
            runtime: tokio::runtime::Builder::new_current_thread().enable_all().build().unwrap(),
        };
        harness.no_update();
        harness
    }

    /// Powers the device on: the ROM picks the firmware, then the system
    /// starts the agent. Returns what the ROM booted.
    fn power_on(&mut self) -> (u8, String) {
        self.power_on_with(&[])
    }

    fn power_on_with(&mut self, env: &[(&str, &str)]) -> (u8, String) {
        assert!(self.agent.is_none(), "already powered on");
        let booted = self.device.rom();
        // The kernel makes boot partitions read-only at every boot
        self.device.lock();
        self.boots += 1;
        self.start_agent(env);
        booted
    }

    /// Starts the agent again without the device rebooting
    fn restart_agent(&mut self) {
        assert!(self.agent.is_none(), "already running");
        self.start_agent(&[]);
    }

    fn start_agent(&mut self, env: &[(&str, &str)]) {
        let log = fs::File::create(&self.log).unwrap();

        let mut cmd = Command::new(env!("CARGO_BIN_EXE_updatehub"));
        cmd.args(["daemon", "-v", "trace", "-c"])
            .arg(&self.setup.settings.stored_path)
            .env("UPDATEHUB_EMMC_SIMULATOR", &self.device.sysfs)
            .env("UPDATEHUB_TEST_BOOT_ID", format!("boot-{}", self.boots))
            .stdout(Stdio::from(log.try_clone().unwrap()))
            .stderr(Stdio::from(log));
        for (k, v) in env {
            cmd.env(k, v);
        }
        // The agent must never outlive the test: once the test is gone, so
        // is the fake `reboot` it relies on, and the next reboot it asks for
        // would be the real one, rebooting the machine running the tests.
        // SAFETY: prctl is async-signal-safe and touches no shared state.
        unsafe {
            cmd.pre_exec(|| {
                nix::sys::prctl::set_pdeathsig(nix::sys::signal::Signal::SIGKILL)?;
                Ok(())
            });
        }
        self.agent = Some(Agent { child: cmd.spawn().unwrap() });

        // A start may reboot right away (a failed validation), or serve
        let deadline = Instant::now() + Duration::from_secs(30);
        while Instant::now() < deadline {
            let agent = &mut self.agent.as_mut().unwrap().child;
            if let Some(status) = agent.try_wait().unwrap() {
                panic!("the agent exited with {status}:\n{}", self.agent_log());
            }
            if self.reboots() > 0 || self.serving() {
                return;
            }
            thread::sleep(Duration::from_millis(100));
        }
        panic!("the agent did not start:\n{}", self.agent_log());
    }

    /// Whether this test's agent answers, not another one on the same port
    fn serving(&self) -> bool {
        let client = sdk::Client::new(&self.listen);
        self.runtime
            .block_on(client.info())
            .is_ok_and(|info| info.runtime_settings.path == self.setup.runtime_settings.stored_path)
    }

    /// The update the device waits to validate, if any
    fn pending(&self) -> Option<String> {
        let runtime: serde_json::Value = serde_json::from_str(
            &fs::read_to_string(&self.setup.runtime_settings.stored_path).ok()?,
        )
        .unwrap();
        runtime["update"]["upgrade_to_installation"].as_str().map(str::to_owned)
    }

    /// The agent rebooted (the `reboot` binary records itself), or the
    /// power was cut: the device is off.
    fn power_off(&mut self) {
        if let Some(mut agent) = self.agent.take() {
            let _ = agent.child.kill();
            let _ = agent.child.wait();
        }
        let output = &self.setup.binaries.data;
        fs::write(output, "").unwrap();
    }

    fn reboots(&self) -> usize {
        fs::read_to_string(&self.setup.binaries.data)
            .unwrap_or_default()
            .lines()
            .filter(|l| l.starts_with("reboot"))
            .count()
    }

    /// Waits for the reboot that installing an update ends with
    fn wait_for_reboot(&mut self) {
        self.wait_for_reboot_after("triggering reboot");
    }

    /// Waits for a reboot, and for the agent to have logged `last` before
    /// it: the log is written asynchronously, and what is still queued when
    /// the power goes off is lost.
    fn wait_for_reboot_after(&mut self, last: &str) {
        let deadline = Instant::now() + Duration::from_secs(60);
        while self.reboots() == 0 {
            assert!(Instant::now() < deadline, "no reboot, agent log:\n{}", self.agent_log());
            thread::sleep(Duration::from_millis(100));
        }
        self.wait_for_log(last);
        self.power_off();
        self.no_update();
    }

    /// The server has nothing for the device, as once it reported the
    /// update installed
    fn no_update(&mut self) {
        for mock in self.mocks.drain(..) {
            mock.remove();
        }
        self.mocks.push(self.server.mock("POST", "/upgrades").with_status(404).create());
    }

    fn wait_for_exit(&mut self) -> ExitStatus {
        let agent = self.agent.as_mut().expect("powered off");
        let deadline = Instant::now() + Duration::from_secs(60);
        let status = loop {
            if let Some(status) = agent.child.try_wait().unwrap() {
                break status;
            }
            assert!(Instant::now() < deadline, "the agent kept running");
            thread::sleep(Duration::from_millis(100));
        };
        self.power_off();
        status
    }

    fn wait_for_log(&self, text: &str) {
        let deadline = Instant::now() + Duration::from_secs(60);
        while !self.agent_log().contains(text) {
            assert!(Instant::now() < deadline, "no {text:?} in agent log:\n{}", self.agent_log());
            thread::sleep(Duration::from_millis(100));
        }
    }

    fn agent_log(&self) -> String {
        fs::read_to_string(&self.log).unwrap_or_default()
    }

    /// Publishes `version`, carrying `firmware`'s images for `target`, and
    /// asks the agent to probe for it.
    fn update(&mut self, version: &str, firmware: &str, target: &Path) {
        let fw = Firmware::new(firmware);
        let object = |name: &str, data: &[u8], seek: usize| {
            json!({
                "mode": "raw",
                "filename": format!("{name}-{firmware}.bin"),
                "size": data.len(),
                "sha256sum": sha256(data),
                "install-if-different": "sha256sum",
                "target-type": "boot-emmc",
                "target": target,
                "chunk-size": CHUNK_SIZE,
                "seek": seek / CHUNK_SIZE,
                "count": -1,
                "truncate": false,
            })
        };
        let set = json!([object("fsbl", &fw.fsbl, 0), object("fip", &fw.fip, FIP_OFFSET)]);
        self.publish(version, set, vec![fw.fsbl.clone(), fw.fip.clone()]);
    }

    /// Publishes `version`, carrying only a system image, and asks the agent
    /// to probe for it.
    fn update_system(&mut self, version: &str) {
        let rootfs = version.as_bytes().repeat(CHUNK_SIZE / version.len());
        let set = json!([{
            "mode": "raw",
            "filename": format!("rootfs-{version}.bin"),
            "size": rootfs.len(),
            "sha256sum": sha256(&rootfs),
            "target-type": "device",
            "target": self.device.rootfs,
            "chunk-size": CHUNK_SIZE,
            "count": -1,
        }]);
        self.publish(version, set, vec![rootfs]);
    }

    fn publish(&mut self, version: &str, set: serde_json::Value, objects: Vec<Vec<u8>>) {
        for mock in self.mocks.drain(..) {
            mock.remove();
        }

        let metadata = json!({
            "product": self.setup.firmware.data.product_uid,
            "version": version,
            "supported-hardware": ["board"],
            "objects": [set, set],
        })
        .to_string();
        let package_uid = sha256(metadata.as_bytes());

        self.mocks.push(
            self.server.mock("POST", "/upgrades").with_status(200).with_body(&metadata).create(),
        );
        for data in &objects {
            self.mocks.push(
                self.server
                    .mock(
                        "GET",
                        format!(
                            "/products/{}/packages/{package_uid}/objects/{}",
                            self.setup.firmware.data.product_uid,
                            sha256(data)
                        )
                        .as_str(),
                    )
                    .with_status(200)
                    .with_body(data)
                    .create(),
            );
        }

        let client = sdk::Client::new(&self.listen);
        self.runtime.block_on(client.probe(None)).expect("probe failed");
    }
}

impl Drop for Harness {
    fn drop(&mut self) {
        self.power_off();
    }
}

fn script(path: &Path, body: &str) {
    fs::write(path, format!("#!/bin/sh\n{body}\n")).unwrap();
    fs::set_permissions(path, fs::Permissions::from_mode(0o755)).unwrap();
}

fn sha256(data: &[u8]) -> String {
    openssl::sha::sha256(data).iter().map(|b| format!("{b:02x}")).collect()
}

#[track_caller]
fn assert_booted(h: &Harness, booted: (u8, String), part: u8, version: &str, set: u8) {
    let log = h.agent_log();
    assert_eq!(booted, (part, version.to_owned()), "ROM booted the wrong firmware\n{log}");
    assert_eq!(h.device.active(), set, "wrong installation set\n{log}");
    assert_eq!(h.device.enabled(), Some(part), "PARTITION_CONFIG moved\n{log}");
}

#[track_caller]
fn assert_locked(h: &Harness) {
    assert_eq!(h.device.force_ro(0), "1", "boot0 left writable");
    assert_eq!(h.device.force_ro(1), "1", "boot1 left writable");
}

#[test]
fn boot_partitions_follow_the_installation_set() {
    let mut h = Harness::new();
    let emmc = h.device.emmc.clone();

    // Factory state: both boot partitions hold 1.0, boot0 enabled, set 0
    let booted = h.power_on();
    assert_booted(&h, booted, 0, "1.0", 0);

    // An update writes the inactive set's boot partition and boots it
    h.update("2.0", "2.0", &emmc);
    h.wait_for_reboot();
    assert_eq!(h.device.firmware(0).as_deref(), Some("1.0"), "the running firmware was touched");
    assert_locked(&h);
    // An agent restarting before the reboot does not validate the update
    h.restart_agent();
    h.wait_for_log("the installed update waits for the device to reboot");
    h.power_off();
    assert_eq!(h.pending().as_deref(), Some("b"), "the update was validated without booting");
    let booted = h.power_on();
    assert_booted(&h, booted, 1, "2.0", 1);
    h.wait_for_log("booting from a recent installation");
    assert_eq!(h.pending(), None);
    h.power_off();

    // A failed validation rolls the firmware back with the system
    fs::write(&h.device.fail_validation, "").unwrap();
    let booted = h.power_on();
    assert_booted(&h, booted, 1, "2.0", 1);
    h.update("3.0", "3.0", &emmc);
    h.wait_for_reboot();
    let booted = h.power_on();
    assert_eq!(booted, (0, "3.0".to_owned()), "the new firmware boots first");
    h.wait_for_reboot_after("swapped active installation set and running rollback");
    fs::remove_file(&h.device.fail_validation).unwrap();
    let booted = h.power_on();
    assert_booted(&h, booted, 1, "2.0", 1);
    h.power_off();

    // A bootloader rollback (boot counter) brings the firmware back
    let _ = h.power_on();
    h.update("4.0", "4.0", &emmc);
    h.wait_for_reboot();
    h.device.set_active(1);
    let booted = h.power_on();
    assert_eq!(booted, (0, "4.0".to_owned()));
    h.wait_for_log("agent start: installation set 1 is active");
    assert_eq!(h.device.enabled(), Some(1), "PARTITION_CONFIG did not follow the set");
    assert_eq!(h.reboots(), 0, "a rolled back update needs no reboot");
    h.power_off();
    let booted = h.power_on();
    assert_booted(&h, booted, 1, "2.0", 1);

    // An update starts from the running firmware: boot0 gets 2.0 over the
    // 4.0 it held, and the same firmware is not written again
    h.update("2.0-r1", "2.0", &emmc);
    h.wait_for_reboot();
    let skipped = h.agent_log().matches("has been skipped (install if different)").count();
    assert_eq!(skipped, 2, "agent log:\n{}", h.agent_log());
    let booted = h.power_on();
    assert_booted(&h, booted, 0, "2.0", 0);
    h.power_off();

    // A power cut while writing the firmware keeps the device booting
    let _ = h.power_on_with(&[("UPDATEHUB_TEST_POWER_CUT_AFTER", "65536")]);
    h.update("5.0", "5.0", &emmc);
    assert_eq!(h.wait_for_exit().signal(), Some(nix::libc::SIGABRT));
    let (old, new) = (Firmware::new("2.0"), Firmware::new("5.0"));
    let boot1 = fs::read(h.device.boot_partition(1)).unwrap();
    let fip = &boot1[FIP_OFFSET..FIP_OFFSET + new.fip.len()];
    assert_eq!(&boot1[..old.fsbl.len()], &old.fsbl[..], "the FSBL was not written yet");
    assert!(fip[..65536] == new.fip[..65536], "the FIP was written up to the power cut");
    assert!(fip[65536..] == old.fip[65536..], "the FIP was written past the power cut");
    let booted = h.power_on();
    assert_booted(&h, booted, 0, "2.0", 0);
    // and the update goes through on the next try
    h.update("5.0", "5.0", &emmc);
    h.wait_for_reboot();
    assert_locked(&h);
    let booted = h.power_on();
    assert_booted(&h, booted, 1, "5.0", 1);

    // A package for another eMMC is refused before anything is written
    h.update("6.0", "6.0", Path::new("/dev/mmcblk7"));
    h.wait_for_log("the device uses");
    assert_eq!(h.reboots(), 0);
    assert_eq!(h.device.firmware(0).as_deref(), Some("2.0"));
    assert_eq!(h.device.enabled(), Some(1));
    h.power_off();

    // A device booting from the boot partition the update writes is left
    // alone on start, and repaired first by the update, with a warning: its
    // firmware moves to the other boot partition, which the eMMC then boots
    // from
    h.device.set_part_config(0x48); // boot0, the inactive set's
    let booted = h.power_on();
    assert_eq!(booted, (0, "2.0".to_owned()));
    assert_eq!(h.device.enabled(), Some(0), "the agent switched without an update pending");
    h.update("6.0", "6.0", &emmc);
    h.wait_for_reboot();
    let log = h.agent_log();
    assert!(log.contains("is about to be overwritten by this update"), "agent log:\n{log}");
    assert_eq!(h.device.firmware(1).as_deref(), Some("2.0"), "the booted firmware was not kept");
    assert_locked(&h);
    let booted = h.power_on();
    assert_booted(&h, booted, 0, "6.0", 0);

    // An update without firmware keeps the running one: boot1 held 2.0
    h.update_system("7.0");
    h.wait_for_reboot();
    assert_locked(&h);
    let booted = h.power_on();
    assert_booted(&h, booted, 1, "6.0", 1);
    h.wait_for_log("booting from a recent installation");
    h.power_off();

    // A power cut between switching the installation set and the eMMC
    // boots the previous firmware once: the agent then switches the eMMC
    // and reboots, so the update is validated with its own firmware
    let _ = h.power_on();
    h.update("8.0", "8.0", &emmc);
    h.wait_for_reboot();
    h.device.set_part_config(0x50); // boot1, as before the update
    let booted = h.power_on();
    assert_eq!(booted, (1, "6.0".to_owned()));
    h.wait_for_reboot_after("rebooting so the update is validated running its own firmware");
    let booted = h.power_on();
    assert_booted(&h, booted, 0, "8.0", 0);
    h.wait_for_log("booting from a recent installation");

    // When the eMMC does not keep booting the update's firmware, the agent
    // reboots for it only once, then rolls the update back
    h.update("9.0", "9.0", &emmc);
    h.wait_for_reboot();
    h.device.set_part_config(0x48); // reset to boot0 on every boot
    let booted = h.power_on();
    assert_eq!(booted, (0, "8.0".to_owned()));
    h.wait_for_reboot_after("rebooting so the update is validated running its own firmware");
    h.device.set_part_config(0x48);
    let booted = h.power_on();
    assert_eq!(booted, (0, "8.0".to_owned()));
    h.wait_for_reboot_after("swapped active installation set and running rollback");
    let booted = h.power_on();
    assert_booted(&h, booted, 0, "8.0", 0);
    h.power_off();

    // A kernel writing its cached PARTITION_CONFIG back once the update
    // writes the boot partition fails the update, and the eMMC goes back to
    // the firmware it was running
    h.device.set_part_config(0x50); // boot1, the inactive set's
    let booted = h.power_on_with(&[("UPDATEHUB_EMMC_SIMULATOR_STALE_KERNEL", "1")]);
    assert_eq!(booted, (1, "9.0".to_owned()));
    h.update("10.0", "10.0", &emmc);
    h.wait_for_log("restored its cached PARTITION_CONFIG");
    assert_eq!(h.reboots(), 0);
    assert_eq!(h.device.enabled(), Some(0), "the eMMC boots the partition just written");
    assert_eq!(h.device.firmware(0).as_deref(), Some("9.0"), "the booted firmware was not kept");
    h.power_off();

    // An update pending from an agent without boot-emmc support never
    // wrote the boot partition of its set, so the eMMC stays where it is
    let _ = h.power_on();
    h.update("11.0", "11.0", &emmc);
    h.wait_for_reboot();
    let path = &h.setup.runtime_settings.stored_path;
    let mut runtime: serde_json::Value =
        serde_json::from_str(&fs::read_to_string(path).unwrap()).unwrap();
    runtime["update"].as_object_mut().unwrap().remove("boot_emmc").unwrap();
    fs::write(path, runtime.to_string()).unwrap();
    h.device.set_part_config(0x48);
    let booted = h.power_on();
    assert_eq!(booted, (0, "9.0".to_owned()));
    h.wait_for_log("booting from a recent installation");
    assert_eq!(
        h.device.enabled(),
        Some(0),
        "the eMMC switched for an update that did not write it"
    );
    assert_eq!(h.pending(), None);
    h.power_off();
}
