// Copyright (C) 2026 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

//! eMMC hardware boot partitions.
//!
//! An eMMC has two boot partitions, `/dev/mmcblkNboot0` and `boot1`, which
//! the kernel keeps read-only (`force_ro`) and one of which the SoC ROM
//! boots from, picked by the BOOT_PARTITION_ENABLE bits of the
//! PARTITION_CONFIG register (EXT_CSD[179]). The register is read and
//! written with the `MMC_IOC_CMD` ioctl.
//!
//! Builds for tests can point this module at a directory instead of the
//! hardware, see [`simulator`].

use super::{Error, Result};
use slog_scope::{info, warn};
use std::{
    fs, io,
    os::unix::fs::OpenOptionsExt,
    path::{Path, PathBuf},
};

const EXT_CSD_PART_CONFIG: usize = 179;
const EXT_CSD_SIZE: usize = 512;
/// Some ROMs need the boot acknowledge, so it is never touched
const BOOT_ACK: u8 = 1 << 6;
/// Set by the kernel to the partition it accesses, so not part of the switch
const PARTITION_ACCESS: u8 = 0b111;

/// The boot partition (0 or 1) a PARTITION_CONFIG value boots from, `None`
/// for the user area or when booting is not enabled.
pub(crate) fn enabled_boot_partition(part_config: u8) -> Option<u8> {
    match (part_config >> 3) & 0b111 {
        1 => Some(0),
        2 => Some(1),
        _ => None,
    }
}

/// PARTITION_CONFIG booting from `partition`, keeping the boot acknowledge
/// of `current`. PARTITION_ACCESS belongs to the kernel, which sets it on
/// every access, so it is left cleared as RAUC and mmc-utils do.
pub(crate) fn part_config_booting(current: u8, partition: u8) -> u8 {
    (current & BOOT_ACK) | ((partition + 1) << 3)
}

/// An eMMC, named by its base device (e.g. `/dev/mmcblk1`)
#[derive(Clone, Debug)]
pub(crate) struct Emmc {
    device: PathBuf,
}

impl Emmc {
    pub(crate) fn new(device: &Path) -> Self {
        Emmc { device: device.to_owned() }
    }

    pub(crate) fn device(&self) -> &Path {
        &self.device
    }

    /// Boot partition `partition` (0 or 1), named after the base device the
    /// configured one resolves to, which may be a symlink such as
    /// `/dev/disk/by-path/...`
    pub(crate) fn boot_partition(&self, partition: u8) -> PathBuf {
        let base = fs::canonicalize(&self.device).unwrap_or_else(|_| self.device.clone());
        let mut path = base.into_os_string();
        path.push(format!("boot{partition}"));
        path.into()
    }

    /// Whether `other` names this eMMC, through a symlink or not
    pub(crate) fn is(&self, other: &Path) -> bool {
        let canonical = |p: &Path| fs::canonicalize(p).unwrap_or_else(|_| p.to_owned());
        canonical(&self.device) == canonical(other)
    }

    pub(crate) fn part_config(&self) -> Result<u8> {
        Ok(backend::read_ext_csd(&self.device)?[EXT_CSD_PART_CONFIG])
    }

    pub(crate) fn enabled_boot_partition(&self) -> Result<Option<u8>> {
        Ok(enabled_boot_partition(self.part_config()?))
    }

    /// Makes the eMMC boot from `partition`, then reads PARTITION_CONFIG
    /// back, as a switch that did not happen must not pass. A kernel that
    /// keeps a cached PARTITION_CONFIG may still write the old value back on
    /// its next boot partition access (see RAUC's documentation on the
    /// ext_csd ioctl bug), so callers check it again after such accesses.
    pub(crate) fn boot_from(&self, partition: u8) -> Result<()> {
        let current = self.part_config()?;
        let wanted = part_config_booting(current, partition);
        if enabled_boot_partition(current) == Some(partition) {
            return Ok(());
        }

        info!(
            "switching {:?} to boot from {:?} (PARTITION_CONFIG 0x{current:02x} -> 0x{wanted:02x})",
            self.device,
            self.boot_partition(partition)
        );
        backend::write_ext_csd(&self.device, EXT_CSD_PART_CONFIG, wanted)?;

        let found = self.part_config()?;
        if found & !PARTITION_ACCESS != wanted {
            return Err(Error::EmmcSwitchFailed {
                device: self.device.clone(),
                expected: wanted,
                found,
            });
        }

        Ok(())
    }

    /// Reads the start of boot partition `partition` from the device, an
    /// access that leaves it as it is.
    pub(crate) fn probe(&self, partition: u8) -> Result<()> {
        read_from_media(&self.boot_partition(partition), 4096)?;
        Ok(())
    }

    /// Copies boot partition `from` over `to`, unless they already hold the
    /// same content. Returns whether it copied.
    pub(crate) fn copy_boot_partition(&self, from: u8, to: u8) -> Result<bool> {
        let source = self.boot_partition(from);
        let target = self.boot_partition(to);
        let data = read_from_media(&source, usize::MAX)?;
        if read_from_media(&target, data.len())? == data {
            return Ok(false);
        }

        let unlocked = ForceRoGuard::unlock(&target)?;
        ensure_writable(&target)?;
        let mut file = fs::OpenOptions::new().write(true).open(&target)?;
        io::Write::write_all(&mut file, &data)?;
        file.sync_all()?;
        drop(file);
        if let Some(unlocked) = unlocked {
            unlocked.lock()?;
        }

        if read_from_media(&target, data.len())? != data {
            return Err(Error::EmmcCopyMismatch { from: source, to: target });
        }

        Ok(true)
    }
}

/// Reads up to `limit` bytes of `path` past the page cache (`O_DIRECT`),
/// which would otherwise answer with what was just written instead of what
/// reached the device.
fn read_from_media(path: &Path, limit: usize) -> Result<Vec<u8>> {
    const ALIGN: usize = 4096;
    const CHUNK: usize = 1024 * 1024;

    simulator::boot_partition_access(path)?;
    if simulator::active() {
        // Simulated boot partitions are regular files, which do not all
        // support O_DIRECT
        let mut data = fs::read(path)?;
        data.truncate(limit);
        return Ok(data);
    }

    let mut file =
        fs::OpenOptions::new().read(true).custom_flags(nix::libc::O_DIRECT).open(path)?;
    let mut buffer = vec![0u8; CHUNK + ALIGN];
    let start = buffer.as_ptr().align_offset(ALIGN);
    let chunk = &mut buffer[start..start + CHUNK];
    let mut data = Vec::new();
    while data.len() < limit {
        let read = io::Read::read(&mut file, chunk)?;
        if read == 0 {
            break;
        }
        data.extend_from_slice(&chunk[..read]);
    }
    data.truncate(limit);
    Ok(data)
}

/// Clears `force_ro` of a block device for as long as it lives, restoring
/// the previous value when dropped.
pub(crate) struct ForceRoGuard {
    path: PathBuf,
    previous: String,
    restored: bool,
}

impl ForceRoGuard {
    /// `None` when the device has no `force_ro` attribute, or it is clear
    /// already: nothing to restore then.
    pub(crate) fn unlock(device: &Path) -> Result<Option<Self>> {
        let path = force_ro_path(device);
        let Ok(previous) = fs::read_to_string(&path) else {
            return Ok(None);
        };
        let previous = previous.trim().to_owned();
        if previous == "0" {
            return Ok(None);
        }

        info!("unlocking {:?} for writing", device);
        fs::write(&path, "0")?;
        Ok(Some(ForceRoGuard { path, previous, restored: false }))
    }

    /// Restores `force_ro`, failing when it cannot: the device must not be
    /// left writable once the update goes on.
    pub(crate) fn lock(mut self) -> Result<()> {
        fs::write(&self.path, &self.previous)?;
        self.restored = true;
        Ok(())
    }
}

impl Drop for ForceRoGuard {
    fn drop(&mut self) {
        if self.restored {
            return;
        }
        if let Err(e) = fs::write(&self.path, &self.previous) {
            warn!("failed to restore {:?} to {}: {}", self.path, self.previous, e);
        }
    }
}

fn force_ro_path(device: &Path) -> PathBuf {
    let device = fs::canonicalize(device).unwrap_or_else(|_| device.to_owned());
    let name = device.file_name().map(PathBuf::from).unwrap_or_default();
    simulator::sysfs_root().join(name).join("force_ro")
}

/// Fails as the kernel would when opening a `force_ro` device for writing.
/// Only the simulator needs this, the kernel enforces it on the hardware.
pub(crate) fn ensure_writable(device: &Path) -> Result<()> {
    if simulator::active()
        && fs::read_to_string(force_ro_path(device)).is_ok_and(|v| v.trim() == "1")
    {
        return Err(io::Error::new(
            io::ErrorKind::PermissionDenied,
            format!("{} is read-only (force_ro)", device.display()),
        )
        .into());
    }
    simulator::boot_partition_access(device)
}

/// Lets tests run the agent against a simulated eMMC.
///
/// With `UPDATEHUB_EMMC_SIMULATOR` naming a directory, which only test
/// builds honour:
/// - the base device is a regular file holding the 512 bytes of EXT_CSD;
/// - the boot partitions are regular files next to it;
/// - `force_ro` lives in `<dir>/<name>/force_ro`, and writes to a boot
///   partition fail while it is set;
/// - with `UPDATEHUB_EMMC_SIMULATOR_STALE_KERNEL` set too, it behaves as a
///   kernel that keeps the PARTITION_CONFIG it read first and writes it back on
///   every boot partition access.
mod simulator {
    use std::path::{Path, PathBuf};

    #[cfg(any(test, feature = "test-env"))]
    pub(super) fn root() -> Option<PathBuf> {
        std::env::var_os("UPDATEHUB_EMMC_SIMULATOR").map(PathBuf::from)
    }

    #[cfg(not(any(test, feature = "test-env")))]
    pub(super) fn root() -> Option<PathBuf> {
        None
    }

    pub(super) fn active() -> bool {
        root().is_some()
    }

    pub(super) fn sysfs_root() -> PathBuf {
        root().unwrap_or_else(|| PathBuf::from("/sys/class/block"))
    }

    #[cfg(any(test, feature = "test-env"))]
    static CACHED_PART_CONFIG: std::sync::OnceLock<u8> = std::sync::OnceLock::new();

    #[cfg(any(test, feature = "test-env"))]
    fn stale_kernel() -> bool {
        active() && std::env::var_os("UPDATEHUB_EMMC_SIMULATOR_STALE_KERNEL").is_some()
    }

    #[cfg(any(test, feature = "test-env"))]
    pub(super) fn part_config_read(value: u8) {
        if stale_kernel() {
            CACHED_PART_CONFIG.get_or_init(|| value);
        }
    }

    #[cfg(not(any(test, feature = "test-env")))]
    pub(super) fn part_config_read(_: u8) {}

    #[cfg(any(test, feature = "test-env"))]
    pub(super) fn boot_partition_access(device: &Path) -> crate::utils::Result<()> {
        let Some(&cached) = CACHED_PART_CONFIG.get().filter(|_| stale_kernel()) else {
            return Ok(());
        };
        let name = device.to_string_lossy();
        let Some(base) = name.strip_suffix("boot0").or_else(|| name.strip_suffix("boot1")) else {
            return Ok(());
        };
        let mut ext_csd = std::fs::read(base)?;
        ext_csd[super::EXT_CSD_PART_CONFIG] = cached;
        std::fs::write(base, ext_csd)?;
        Ok(())
    }

    #[cfg(not(any(test, feature = "test-env")))]
    pub(super) fn boot_partition_access(_: &Path) -> crate::utils::Result<()> {
        Ok(())
    }
}

mod backend {
    use super::{EXT_CSD_SIZE, simulator};
    use crate::utils::Result;
    use std::{fs, path::Path};

    pub(super) fn read_ext_csd(device: &Path) -> Result<[u8; EXT_CSD_SIZE]> {
        if simulator::active() {
            let mut ext_csd = [0; EXT_CSD_SIZE];
            let data = fs::read(device)?;
            let len = data.len().min(EXT_CSD_SIZE);
            ext_csd[..len].copy_from_slice(&data[..len]);
            simulator::part_config_read(ext_csd[super::EXT_CSD_PART_CONFIG]);
            return Ok(ext_csd);
        }
        ioctl::read_ext_csd(device)
    }

    pub(super) fn write_ext_csd(device: &Path, index: usize, value: u8) -> Result<()> {
        if simulator::active() {
            let mut ext_csd = read_ext_csd(device)?;
            ext_csd[index] = value;
            fs::write(device, ext_csd)?;
            return Ok(());
        }
        ioctl::write_ext_csd(device, index, value)
    }

    /// `MMC_IOC_CMD`, as in `linux/mmc/ioctl.h` and mmc-utils
    mod ioctl {
        use super::EXT_CSD_SIZE;
        use crate::utils::Result;
        use std::{fs, os::fd::AsRawFd, path::Path};

        const MMC_BLOCK_MAJOR: u8 = 179;

        const MMC_RSP_PRESENT: u32 = 1 << 0;
        const MMC_RSP_CRC: u32 = 1 << 2;
        const MMC_RSP_BUSY: u32 = 1 << 3;
        const MMC_RSP_OPCODE: u32 = 1 << 4;
        const MMC_CMD_AC: u32 = 0 << 5;
        const MMC_CMD_ADTC: u32 = 1 << 5;
        const MMC_RSP_SPI_S1: u32 = 1 << 7;
        const MMC_RSP_SPI_BUSY: u32 = 1 << 10;

        const MMC_RSP_R1: u32 = MMC_RSP_PRESENT | MMC_RSP_CRC | MMC_RSP_OPCODE;
        const MMC_RSP_R1B: u32 = MMC_RSP_R1 | MMC_RSP_BUSY;
        const MMC_RSP_SPI_R1: u32 = MMC_RSP_SPI_S1;
        const MMC_RSP_SPI_R1B: u32 = MMC_RSP_SPI_S1 | MMC_RSP_SPI_BUSY;

        const MMC_SWITCH: u32 = 6;
        const MMC_SEND_EXT_CSD: u32 = 8;
        const MMC_SWITCH_MODE_WRITE_BYTE: u32 = 0x03;
        const EXT_CSD_CMD_SET_NORMAL: u32 = 1;

        #[repr(C)]
        #[derive(Default)]
        pub(super) struct MmcIocCmd {
            write_flag: i32,
            is_acmd: i32,
            opcode: u32,
            arg: u32,
            response: [u32; 4],
            flags: u32,
            blksz: u32,
            blocks: u32,
            postsleep_min_us: u32,
            postsleep_max_us: u32,
            data_timeout_ns: u32,
            cmd_timeout_ms: u32,
            pad: u32,
            data_ptr: u64,
        }

        const _: () = assert!(std::mem::size_of::<MmcIocCmd>() == 72);

        nix::ioctl_readwrite!(mmc_ioc_cmd, MMC_BLOCK_MAJOR, 0, MmcIocCmd);

        fn run(device: &Path, cmd: &mut MmcIocCmd) -> Result<()> {
            let file = fs::OpenOptions::new().read(true).write(true).open(device)?;
            // SAFETY: `cmd` matches the kernel's `struct mmc_ioc_cmd`, and its
            // data pointer, when set, points at a buffer of `blksz * blocks`
            // bytes the caller keeps alive across the call.
            unsafe { mmc_ioc_cmd(file.as_raw_fd(), cmd) }?;
            Ok(())
        }

        pub(super) fn read_ext_csd(device: &Path) -> Result<[u8; EXT_CSD_SIZE]> {
            let mut ext_csd = [0; EXT_CSD_SIZE];
            let mut cmd = MmcIocCmd {
                opcode: MMC_SEND_EXT_CSD,
                flags: MMC_RSP_SPI_R1 | MMC_RSP_R1 | MMC_CMD_ADTC,
                blksz: EXT_CSD_SIZE as u32,
                blocks: 1,
                data_ptr: ext_csd.as_mut_ptr() as u64,
                ..Default::default()
            };
            run(device, &mut cmd)?;
            Ok(ext_csd)
        }

        pub(super) fn write_ext_csd(device: &Path, index: usize, value: u8) -> Result<()> {
            let mut cmd = MmcIocCmd {
                write_flag: 1,
                opcode: MMC_SWITCH,
                arg: (MMC_SWITCH_MODE_WRITE_BYTE << 24)
                    | ((index as u32) << 16)
                    | (u32::from(value) << 8)
                    | EXT_CSD_CMD_SET_NORMAL,
                flags: MMC_RSP_SPI_R1B | MMC_RSP_R1B | MMC_CMD_AC,
                ..Default::default()
            };
            run(device, &mut cmd)
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;

    #[test]
    fn enabled_partition() {
        assert_eq!(enabled_boot_partition(0x00), None);
        assert_eq!(enabled_boot_partition(0x08), Some(0));
        assert_eq!(enabled_boot_partition(0x48), Some(0));
        assert_eq!(enabled_boot_partition(0x50), Some(1));
        assert_eq!(enabled_boot_partition(0x38), None, "user area");
        assert_eq!(enabled_boot_partition(0x51), Some(1), "PARTITION_ACCESS is ignored");
    }

    #[test]
    fn switching_keeps_only_the_boot_ack() {
        assert_eq!(part_config_booting(0x48, 1), 0x50);
        assert_eq!(part_config_booting(0x50, 0), 0x48);
        assert_eq!(part_config_booting(0x08, 1), 0x10);
        // PARTITION_ACCESS (bits 0-2) and reserved bit 7 are not carried over
        assert_eq!(part_config_booting(0xc9, 1), 0x50);
    }
}
