// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Error, Result};
use crate::utils::definitions::IdExt;
use pkg_schema::definitions::{
    Filesystem,
    target_permissions::{Gid, Uid},
};
use slog_scope::trace;
use std::{io, io::Seek, os::unix::fs::FileTypeExt, path::Path};
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

pub(crate) fn is_executable_in_path(cmd: &str) -> Result<()> {
    trace!("checking if {} is executable", cmd);
    match quale::which(cmd) {
        Some(_) => Ok(()),
        None => Err(Error::ExecutableNotInPath(cmd.to_owned())),
    }
}

pub(crate) fn format(target: &Path, fs: Filesystem, options: &Option<String>) -> Result<()> {
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
}
