// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Error, Result};
use crate::utils::{definitions::IdExt, mtd};
use pkg_schema::definitions::{
    Filesystem,
    target_permissions::{Gid, Uid},
};
use slog_scope::trace;
use std::{
    collections::HashSet,
    io::{self, Seek},
    os::unix::fs::FileTypeExt,
    path::{Path, PathBuf},
};
use sys_mount::{Mount, Unmount, UnmountDrop};

pub(crate) struct MountGuard {
    _mount: UnmountDrop<Mount>,
    directory: tempfile::TempDir,
}

impl MountGuard {
    pub(crate) fn mount_point(&self) -> &Path {
        self.directory.path()
    }
}

pub(crate) fn ensure_disk_space(target: &Path, required: u64) -> Result<()> {
    trace!("looking for {} free bytes on {:?}", required, target);
    let available = available_space(target)?;

    if required > available {
        return Err(Error::NotEnoughSpace { available, required });
    }
    Ok(())
}

/// Returns how many bytes may be written to `target`.
///
/// `statvfs` reports on the filesystem holding the given path, so pointing it
/// at a device node measures the devtmpfs mounted on `/dev` -- which is sized
/// after the available RAM -- instead of the device the objects are installed
/// onto. Device nodes are therefore asked for their own capacity, which a seek
/// to the end reports for block, MTD and UBI volume devices alike.
fn available_space(target: &Path) -> Result<u64> {
    let file_type = std::fs::metadata(target)?.file_type();

    if file_type.is_block_device() || file_type.is_char_device() {
        trace!("{:?} is a device node, asking it for its capacity", target);
        return Ok(std::fs::File::open(target)?.seek(io::SeekFrom::End(0))?);
    }

    let stat = nix::sys::statvfs::statvfs(target)?;

    // stat fields might be 32 or 64 bits wide depending on the host arch, so widen
    // them before multiplying to keep filesystems bigger than 4 GiB from
    // overflowing on 32 bit targets
    #[allow(clippy::useless_conversion)]
    Ok(u64::from(stat.fragment_size()) * u64::from(stat.blocks_free()))
}

/// Block device number, as `(major, minor)`.
type DeviceId = (u64, u64);

/// Ensures no mounted filesystem lives on `target`, so an installation writing
/// to it cannot corrupt data the running system still believes it owns.
///
/// Besides `target` itself this covers the partitions it contains, the disk it
/// is a partition of and whatever is stacked on top of it, as damaging any of
/// those damages the others.
pub(crate) fn ensure_not_mounted(target: &Path) -> Result<()> {
    trace!("checking whether {:?} is in use", target);

    let metadata = match std::fs::metadata(target) {
        Ok(metadata) => metadata,
        // Nothing can be mounted on a device which is not there. Complaining
        // about it is left to the existence check.
        Err(e) if e.kind() == io::ErrorKind::NotFound => return Ok(()),
        Err(e) => return Err(e.into()),
    };

    // MTD and UBI offer no block device to compare against, they are named in
    // `/proc/self/mountinfo` instead.
    let sources = mtd::mount_sources_for(target).into_iter().collect::<HashSet<_>>();

    let mut devices = HashSet::new();
    devices.extend(block_device_id(&metadata).into_iter().flat_map(related_block_devices));
    for source in sources.iter().filter(|source| source.starts_with('/')) {
        if let Ok(metadata) = std::fs::metadata(source) {
            devices.extend(block_device_id(&metadata).into_iter().flat_map(related_block_devices));
        }
    }

    let mountinfo = std::fs::read_to_string("/proc/self/mountinfo")?;
    if let Some(entry) = mountinfo
        .lines()
        .filter_map(parse_mount_entry)
        .find(|entry| devices.contains(&entry.device) || sources.contains(&entry.source))
    {
        return Err(Error::DeviceInUse {
            device: target.to_owned(),
            mount_point: entry.mount_point,
        });
    }

    Ok(())
}

fn block_device_id(metadata: &std::fs::Metadata) -> Option<DeviceId> {
    use std::os::unix::fs::MetadataExt;

    metadata
        .file_type()
        .is_block_device()
        .then(|| (nix::sys::stat::major(metadata.rdev()), nix::sys::stat::minor(metadata.rdev())))
}

/// An entry of `/proc/self/mountinfo`.
struct MountEntry {
    device: DeviceId,
    source: String,
    mount_point: PathBuf,
}

/// Parses a line of `/proc/self/mountinfo`:
///
/// ```text
/// 26 25 8:2 / /boot rw,relatime shared:5 - ext4 /dev/sda2 rw
/// ```
///
/// The optional fields before the `-` separator vary in count, so the mount
/// source is located relative to it rather than by a fixed index.
fn parse_mount_entry(line: &str) -> Option<MountEntry> {
    let fields = line.split_whitespace().collect::<Vec<_>>();
    let separator = fields.iter().position(|field| *field == "-")?;
    let (major, minor) = fields.get(2)?.split_once(':')?;

    Some(MountEntry {
        device: (major.parse().ok()?, minor.parse().ok()?),
        source: fields.get(separator + 2)?.to_string(),
        mount_point: PathBuf::from(fields.get(4)?),
    })
}

/// Collects `device` along with every block device sharing its storage: the
/// partitions it contains, the disk it is a partition of and whatever is
/// stacked on top of it, such as LVM, MD and loop devices.
///
/// Sibling partitions are deliberately left out, writing to one partition does
/// not reach the others.
fn related_block_devices(device: DeviceId) -> HashSet<DeviceId> {
    let mut found = HashSet::new();
    let mut pending = vec![device];

    while let Some(device) = pending.pop() {
        if !found.insert(device) {
            continue;
        }

        let sysfs = PathBuf::from(format!("/sys/dev/block/{}:{}", device.0, device.1));
        let Ok(sysfs) = sysfs.canonicalize() else {
            continue;
        };

        // A partition's directory lives inside the one of its disk. The disk is
        // recorded but not walked, otherwise its other partitions would be
        // dragged in as well.
        if sysfs.join("partition").exists() {
            found.extend(sysfs.parent().and_then(|disk| read_device_id(&disk.join("dev"))));
        } else {
            pending.extend(device_ids_in(&sysfs, |path| path.join("partition").exists()));
        }

        pending.extend(device_ids_in(&sysfs.join("holders"), |_| true));
    }

    found
}

/// Device numbers of the sysfs entries under `dir` which satisfy `wanted`.
fn device_ids_in(dir: &Path, wanted: impl Fn(&Path) -> bool) -> Vec<DeviceId> {
    let Ok(entries) = std::fs::read_dir(dir) else {
        return Vec::default();
    };

    entries
        .filter_map(std::result::Result::ok)
        .map(|entry| entry.path())
        .filter(|path| wanted(path))
        .filter_map(|path| read_device_id(&path.join("dev")))
        .collect()
}

fn read_device_id(path: &Path) -> Option<DeviceId> {
    let content = std::fs::read_to_string(path).ok()?;
    let (major, minor) = content.trim().split_once(':')?;

    Some((major.parse().ok()?, minor.parse().ok()?))
}

pub(crate) fn is_executable_in_path(cmd: &str) -> Result<()> {
    trace!("checking if {} is executable", cmd);
    match quale::which(cmd) {
        Some(_) => Ok(()),
        None => Err(Error::ExecutableNotInPath(cmd.to_owned())),
    }
}

pub(crate) fn format(target: &Path, fs: Filesystem, options: &Option<String>) -> Result<()> {
    // The commands below are forced so they run unattended, which also means
    // they will not refuse to wipe a mounted filesystem on their own.
    ensure_not_mounted(target)?;

    trace!("formating {:?} as {}", target, fs);
    let target = target.display();
    let options = options.clone().unwrap_or_default();

    let cmd = match fs {
        Filesystem::Jffs2 => format!("flash_erase -j {options} {target} 0 0"),
        Filesystem::Ext2 | Filesystem::Ext3 | Filesystem::Ext4 => {
            format!("mkfs.{fs} -F {options} {target}")
        }
        Filesystem::Ubifs => format!("mkfs.{fs} -y {options} {target}"),
        Filesystem::Xfs => format!("mkfs.{fs} -f {options} {target}"),
        Filesystem::Btrfs | Filesystem::Vfat | Filesystem::F2fs => {
            format!("mkfs.{fs} {options} {target}")
        }
    };

    easy_process::run(&cmd)?;
    Ok(())
}

pub(crate) fn mount(source: &Path, fs: Filesystem, options: &str) -> io::Result<MountGuard> {
    let directory = tempfile::tempdir()?;
    let dest = directory.path();

    trace!("mounting {:?} as {} at {:?}", source, fs, &dest);

    let _mount = Mount::builder()
        .fstype(format!("{fs}").as_str())
        .data(options)
        .flags(sys_mount::MountFlags::empty())
        .mount(source, dest)?
        .into_unmount_drop(sys_mount::UnmountFlags::FORCE);

    Ok(MountGuard { _mount, directory })
}

pub(crate) fn chmod(path: &Path, mode: u32) -> Result<()> {
    trace!("applying 0o{:o} permissions to {:?}", mode, path);
    nix::sys::stat::fchmodat(
        nix::fcntl::AT_FDCWD,
        path,
        nix::sys::stat::Mode::from_bits(mode).unwrap(),
        nix::sys::stat::FchmodatFlags::FollowSymlink,
    )?;

    Ok(())
}

pub(crate) fn chown(path: &Path, uid: &Option<Uid>, gid: &Option<Gid>) -> Result<()> {
    trace!("applying ownership of uid:{:?} and gid:{:?} to {:?}", uid, gid, path);
    Ok(nix::unistd::chown(
        path,
        uid.as_ref().map(|id| nix::unistd::Uid::from_raw(id.as_u32())),
        gid.as_ref().map(|id| nix::unistd::Gid::from_raw(id.as_u32())),
    )?)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::object::installer::tests::SERIALIZE;
    use pretty_assertions::assert_eq;
    use std::path::PathBuf;

    // Big enough to tell a loop device apart from the devtmpfs on /dev, small
    // enough to stay sparse on any builder.
    const LOOP_DEVICE_SIZE: u64 = 64 * 1024 * 1024;

    struct FakeLoopDevice {
        loopdev: loopdev::LoopDevice,
        device: PathBuf,
        _backing_file: tempfile::NamedTempFile,
    }

    impl FakeLoopDevice {
        fn new(size: u64) -> Result<FakeLoopDevice> {
            let backing_file = tempfile::NamedTempFile::new()?;
            backing_file.as_file().set_len(size)?;

            // Loop device next_free is not thread safe
            let mutex = SERIALIZE.clone();
            let _mutex = mutex.lock().unwrap();
            let loopdev = loopdev::LoopControl::open()?.next_free()?;
            let device = loopdev.path().unwrap();
            loopdev.attach_file(backing_file.path())?;

            Ok(FakeLoopDevice { loopdev, device, _backing_file: backing_file })
        }
    }

    impl Drop for FakeLoopDevice {
        fn drop(&mut self) {
            if let Err(e) = self.loopdev.detach() {
                eprintln!("Failed to cleanup FakeLoopDevice, Error: {e}");
            }
        }
    }

    #[test]
    fn regular_files_are_measured_by_their_filesystem() {
        let dir = tempfile::tempdir().unwrap();
        let file = dir.path().join("object");
        std::fs::write(&file, [0; 16]).unwrap();

        // The seek a device node is asked for would answer with the file's own
        // 16 bytes instead.
        assert!(available_space(&file).unwrap() > 16);
    }

    #[test]
    fn a_requirement_beyond_the_available_space_is_rejected() {
        let dir = tempfile::tempdir().unwrap();
        let available = available_space(dir.path()).unwrap();

        ensure_disk_space(dir.path(), available).unwrap();
        assert!(matches!(
            ensure_disk_space(dir.path(), available + 1),
            Err(Error::NotEnoughSpace { .. })
        ));
    }

    // Requires root, as creating a loop device does.
    #[test]
    #[ignore]
    fn device_nodes_are_measured_by_their_own_capacity() {
        let loop_device = FakeLoopDevice::new(LOOP_DEVICE_SIZE).unwrap();

        // Asking for the directory holding the node is asking the devtmpfs mounted
        // on /dev, which is the answer the check used to settle for.
        assert_ne!(available_space(Path::new("/dev")).unwrap(), LOOP_DEVICE_SIZE);

        assert_eq!(available_space(&loop_device.device).unwrap(), LOOP_DEVICE_SIZE);
        ensure_disk_space(&loop_device.device, LOOP_DEVICE_SIZE).unwrap();
        assert!(matches!(
            ensure_disk_space(&loop_device.device, LOOP_DEVICE_SIZE + 1),
            Err(Error::NotEnoughSpace { .. })
        ));
    }

    #[test]
    fn mount_entry_without_optional_fields() {
        let entry = parse_mount_entry("26 25 8:2 / /boot rw,relatime - ext4 /dev/sda2 rw").unwrap();

        assert_eq!(entry.device, (8, 2));
        assert_eq!(entry.source, "/dev/sda2");
        assert_eq!(entry.mount_point, PathBuf::from("/boot"));
    }

    #[test]
    fn mount_entry_with_optional_fields() {
        let entry = parse_mount_entry(
            "26 25 0:23 / / rw,relatime shared:5 master:1 - ubifs ubi0:rootfs rw",
        )
        .unwrap();

        assert_eq!(entry.device, (0, 23));
        assert_eq!(entry.source, "ubi0:rootfs");
        assert_eq!(entry.mount_point, PathBuf::from("/"));
    }

    #[test]
    fn malformed_mount_entries_are_dropped() {
        assert!(parse_mount_entry("").is_none());
        assert!(parse_mount_entry("26 25 8:2 / /boot rw,relatime ext4 /dev/sda2 rw").is_none());
        assert!(parse_mount_entry("26 25 bogus / /boot rw - ext4 /dev/sda2 rw").is_none());
    }

    #[test]
    fn regular_file_is_not_in_use() {
        let file = tempfile::NamedTempFile::new().unwrap();

        assert!(ensure_not_mounted(file.path()).is_ok());
    }

    #[test]
    fn missing_device_is_not_in_use() {
        assert!(ensure_not_mounted(Path::new("/dev/updatehub-inexistent-device")).is_ok());
    }

    // Requires root, as creating a loop device does.
    #[test]
    #[ignore]
    fn mounted_device_is_in_use() {
        let loop_device = FakeLoopDevice::new(LOOP_DEVICE_SIZE).unwrap();

        assert!(
            ensure_not_mounted(&loop_device.device).is_ok(),
            "an unmounted loop device should be free to install onto"
        );

        format(&loop_device.device, Filesystem::Ext4, &None).unwrap();
        let guard = mount(&loop_device.device, Filesystem::Ext4, "").unwrap();

        let in_use = ensure_not_mounted(&loop_device.device);
        // Unmount before asserting so the device is always released.
        drop(guard);

        assert!(
            matches!(in_use, Err(Error::DeviceInUse { .. })),
            "a mounted device must be rejected, got {in_use:?}"
        );
    }
}
