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
        source: (*fields.get(separator + 2)?).to_string(),
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

    #[cfg(test)]
    if let Some(dirs) = search_path::current() {
        return if dirs.iter().any(|dir| is_executable_file(&dir.join(cmd))) {
            Ok(())
        } else {
            Err(Error::ExecutableNotInPath(cmd.to_owned()))
        };
    }

    match quale::which(cmd) {
        Some(_) => Ok(()),
        None => Err(Error::ExecutableNotInPath(cmd.to_owned())),
    }
}

#[cfg(test)]
fn is_executable_file(path: &Path) -> bool {
    use std::os::unix::fs::PermissionsExt;

    // `metadata` follows symlinks, as an execution attempt does.
    std::fs::metadata(path)
        .map(|m| m.is_file() && m.permissions().mode() & 0o111 != 0)
        .unwrap_or(false)
}

/// A per-thread override for the directories `is_executable_in_path` reads,
/// so a test needs no change to `PATH`. It works because libtest gives each
/// test its own thread and every test here uses the current-thread runtime.
///
/// Use `super::test_env::PathEnvGuard` instead when the test runs a real
/// subprocess, because `execvp` reads `PATH` from the process environment.
#[cfg(test)]
pub(crate) mod search_path {
    use std::{cell::RefCell, path::PathBuf};

    thread_local! {
        static OVERRIDE: RefCell<Option<Vec<PathBuf>>> = const { RefCell::new(None) };
    }

    /// Restores the previous override on drop. Guards nest: an inner one
    /// hides the outer one until it drops.
    #[must_use = "the override ends as soon as the guard drops"]
    pub(crate) struct SearchPathGuard {
        previous: Option<Vec<PathBuf>>,
    }

    impl SearchPathGuard {
        pub(crate) fn set<I, P>(dirs: I) -> Self
        where
            I: IntoIterator<Item = P>,
            P: Into<PathBuf>,
        {
            let dirs = dirs.into_iter().map(Into::into).collect();
            let previous = OVERRIDE.with_borrow_mut(|o| o.replace(dirs));

            SearchPathGuard { previous }
        }

        pub(crate) fn empty() -> Self {
            Self::set(Vec::<PathBuf>::new())
        }
    }

    impl Drop for SearchPathGuard {
        fn drop(&mut self) {
            OVERRIDE.with_borrow_mut(|o| *o = self.previous.take());
        }
    }

    pub(super) fn current() -> Option<Vec<PathBuf>> {
        OVERRIDE.with_borrow(std::clone::Clone::clone)
    }
}

pub(crate) fn format(target: &Path, fs: Filesystem, options: Option<&str>) -> Result<()> {
    // The commands below are forced so they run unattended, which also means
    // they will not refuse to wipe a mounted filesystem on their own.
    ensure_not_mounted(target)?;

    trace!("formating {:?} as {}", target, fs);
    let target = target.display();
    let options = options.unwrap_or_default();

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

pub(crate) fn chown(path: &Path, uid: Option<&Uid>, gid: Option<&Gid>) -> Result<()> {
    trace!("applying ownership of uid:{:?} and gid:{:?} to {:?}", uid, gid, path);
    Ok(nix::unistd::chown(
        path,
        uid.map(|id| nix::unistd::Uid::from_raw(id.as_u32())),
        gid.map(|id| nix::unistd::Gid::from_raw(id.as_u32())),
    )?)
}

#[cfg(test)]
mod tests {
    use super::{search_path::SearchPathGuard, *};
    use crate::object::installer::tests::SERIALIZE;
    use pretty_assertions::assert_eq;
    use std::path::PathBuf;

    fn create_executable(name: &str) -> tempfile::TempDir {
        use std::os::unix::fs::PermissionsExt;

        let dir = tempfile::tempdir().unwrap();
        let file = std::fs::File::create(dir.path().join(name)).unwrap();
        file.set_permissions(std::fs::Permissions::from_mode(0o755)).unwrap();

        dir
    }

    #[test]
    fn search_path_override_replaces_the_process_path() {
        // `sh` is on the real PATH of every host that runs this suite.
        assert!(is_executable_in_path("sh").is_ok());

        let dir = create_executable("updatehub-fake-tool");

        let guard = SearchPathGuard::set([dir.path()]);
        assert!(
            is_executable_in_path("updatehub-fake-tool").is_ok(),
            "the override has to make the tool visible"
        );
        assert!(is_executable_in_path("sh").is_err(), "the override has to hide the real PATH");
        drop(guard);

        assert!(is_executable_in_path("sh").is_ok(), "the drop has to restore the real PATH");
        assert!(is_executable_in_path("updatehub-fake-tool").is_err());
    }

    #[test]
    fn search_path_override_finds_nothing_when_empty() {
        let _guard = SearchPathGuard::empty();
        assert!(is_executable_in_path("sh").is_err());
    }

    #[test]
    fn search_path_override_nests() {
        let outer_dir = create_executable("updatehub-outer-tool");
        let inner_dir = create_executable("updatehub-inner-tool");

        let _outer = SearchPathGuard::set([outer_dir.path()]);
        assert!(is_executable_in_path("updatehub-outer-tool").is_ok());

        let inner = SearchPathGuard::set([inner_dir.path()]);
        assert!(is_executable_in_path("updatehub-inner-tool").is_ok());
        assert!(is_executable_in_path("updatehub-outer-tool").is_err());
        drop(inner);

        assert!(is_executable_in_path("updatehub-outer-tool").is_ok());
        assert!(is_executable_in_path("updatehub-inner-tool").is_err());
    }

    #[test]
    fn search_path_override_skips_a_file_without_the_executable_bit() {
        let dir = tempfile::tempdir().unwrap();
        std::fs::write(dir.path().join("updatehub-plain-file"), b"").unwrap();

        let _guard = SearchPathGuard::set([dir.path()]);
        assert!(is_executable_in_path("updatehub-plain-file").is_err());
    }

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

        // `ensure_disk_space` reads the free space itself, and anything else
        // writing to the same filesystem moves it between the two readings. Stay
        // away from the boundary on both sides: ask for half of what is there,
        // then for more than any filesystem can hold.
        ensure_disk_space(dir.path(), available / 2).unwrap();
        assert!(matches!(
            ensure_disk_space(dir.path(), u64::MAX),
            Err(Error::NotEnoughSpace { .. })
        ));
    }

    #[test]
    #[ignore = "needs root to attach a loop device"]
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

    #[test]
    #[ignore = "needs root to attach a loop device"]
    fn mounted_device_is_in_use() {
        let loop_device = FakeLoopDevice::new(LOOP_DEVICE_SIZE).unwrap();

        assert!(
            ensure_not_mounted(&loop_device.device).is_ok(),
            "an unmounted loop device should be free to install onto"
        );

        format(&loop_device.device, Filesystem::Ext4, None).unwrap();
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
