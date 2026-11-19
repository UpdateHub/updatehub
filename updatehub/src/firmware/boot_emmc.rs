// Copyright (C) 2026 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

//! A/B firmware in the eMMC hardware boot partitions.
//!
//! With `[firmware.boot-emmc]` set, installation set N boots from boot
//! partition N: updates copy the running firmware to the boot partition of
//! the inactive set and write their raw objects with the `boot-emmc` target
//! type over it, and the eMMC boots from the boot partition of the active
//! set.
//!
//! The eMMC only switches to a boot partition after the update wrote and
//! synced it, so a power cut while writing leaves the device booting the
//! firmware it had. The installation set and PARTITION_CONFIG are two
//! separate writes, though: a power cut between them boots the previous
//! firmware with the new set once, and the agent then switches the eMMC and
//! reboots before validating, so the new firmware gets its trial boot. It
//! reboots for that only once per update: when the eMMC still does not boot
//! the update's firmware after it, the update is rolled back.

use super::{Error, Result, installation_set};
use crate::{settings::Settings, utils::emmc::Emmc};
use slog_scope::{error, info, warn};

pub(crate) fn emmc(settings: &Settings) -> Option<Emmc> {
    settings.firmware.boot_emmc.as_ref().map(|c| Emmc::new(&c.device))
}

/// Makes the eMMC boot from the boot partition of the active installation
/// set, returning whether it had to switch. `reason` tells why the set
/// changed, for the log.
pub(crate) fn follow_active_set(settings: &Settings, reason: &str) -> Result<bool> {
    let Some(emmc) = emmc(settings) else {
        return Ok(false);
    };
    let set = installation_set::active()?.index();

    match enabled(&emmc)? {
        partition if partition == set => Ok(false),
        partition => {
            info!(
                "{reason}: installation set {set} is active while {:?} boots from {:?}",
                emmc.device(),
                emmc.boot_partition(partition)
            );
            emmc.boot_from(set)?;
            Ok(true)
        }
    }
}

/// Whether the eMMC boots from the boot partition of the active
/// installation set, or no boot-emmc device is set.
pub(crate) fn follows_active_set(settings: &Settings) -> Result<bool> {
    match emmc(settings) {
        Some(emmc) => Ok(enabled(&emmc)? == installation_set::active()?.index()),
        None => Ok(true),
    }
}

/// Before an update writes installation set `target`, which `pending` must
/// not be waiting on a reboot to boot.
///
/// The eMMC must not be booting from the boot partition of `target`. A
/// device that is, such as one set up before the boot partitions followed
/// the installation sets, is repaired by copying that firmware to the other
/// boot partition and booting from there.
///
/// The running firmware is then copied to `target`'s boot partition, so
/// what the update does not write, if anything, is the firmware running
/// now rather than whatever an older or interrupted update left there.
///
/// The update's state must be persistent: without it, nothing would bring
/// the eMMC back after a rollback.
pub(crate) fn prepare_install(
    settings: &Settings,
    persistent: bool,
    pending: Option<installation_set::Set>,
    target: u8,
) -> Result<()> {
    let Some(emmc) = emmc(settings) else {
        return Ok(());
    };
    if !persistent {
        return Err(Error::EmmcStateNotPersistent);
    }
    if let Some(pending) = pending {
        return Err(Error::EmmcRebootPending(pending.index()));
    }
    if installation_set::active()?.index() == target {
        return Err(Error::EmmcTargetActive(target));
    }

    let mut running = enabled(&emmc)?;
    if running == target {
        let other = 1 - target;
        warn!(
            "{:?} boots from {:?}, which belongs to installation set {target} and is about to \
             be overwritten by this update: copying its firmware to {:?}, the boot partition \
             of installation set {other}, and booting from there instead",
            emmc.device(),
            emmc.boot_partition(target),
            emmc.boot_partition(other),
        );
        emmc.probe(other)?;
        ensure_not_booting(&emmc, other)?;
        emmc.copy_boot_partition(target, other)?;
        emmc.boot_from(other)?;
        warn!("{:?} now boots from {:?}", emmc.device(), emmc.boot_partition(other));
        running = other;
    }

    // A kernel restoring its cached PARTITION_CONFIG does so on the next
    // access to a boot partition. Before writing one, that access must be a
    // read, which leaves it as it is if the eMMC then boots from it.
    emmc.probe(target)?;
    ensure_not_booting(&emmc, target)?;

    if emmc.copy_boot_partition(running, target)? {
        info!(
            "copied the running firmware from {:?} to {:?}",
            emmc.boot_partition(running),
            emmc.boot_partition(target)
        );
    }

    Ok(())
}

/// After an update wrote the boot partition of installation set `target`:
/// the eMMC must still boot from the other one.
pub(crate) fn check_installed(settings: &Settings, target: u8) -> Result<()> {
    match emmc(settings) {
        Some(emmc) => ensure_not_booting(&emmc, target),
        None => Ok(()),
    }
}

/// A kernel restoring its cached PARTITION_CONFIG on an access to the boot
/// partition of `target` makes the eMMC boot from it, so it is switched
/// back before failing.
fn ensure_not_booting(emmc: &Emmc, target: u8) -> Result<()> {
    if enabled(emmc)? == target {
        if let Err(e) = emmc.boot_from(1 - target) {
            error!("unable to switch {:?} back to the running firmware: {}", emmc.device(), e);
        }
        return Err(Error::EmmcSwitchReverted {
            device: emmc.device().to_owned(),
            partition: emmc.boot_partition(target),
        });
    }
    Ok(())
}

fn enabled(emmc: &Emmc) -> Result<u8> {
    match emmc.enabled_boot_partition()? {
        Some(partition) => Ok(partition),
        None => Err(Error::EmmcNotBooting {
            device: emmc.device().to_owned(),
            part_config: emmc.part_config()?,
        }),
    }
}
