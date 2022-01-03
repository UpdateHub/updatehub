// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Error, Result};
use crate::utils::definitions::IdExt;
use pkg_schema::definitions::{
    target_permissions::{Gid, Uid},
    Filesystem,
};
use slog_scope::trace;
use std::{
    io,
    path::{Path, PathBuf},
};
use sys_mount::{Mount, Unmount, UnmountDrop};

pub(crate) fn ensure_disk_space(target: &Path, required: u64) -> Result<()> {
    trace!("looking for {} free bytes on {:?}", required, target);
    let stat = nix::sys::statvfs::statvfs(target)?;

    // stat fields might be 32 or 64 bytes depending on host arch
    let available = stat.block_size() as u64 * stat.blocks_free() as u64;

    if required > available {
        return Err(Error::NotEnoughSpace { available, required });
    }
    Ok(())
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
    let options = options.clone().unwrap_or_else(|| "".to_string());

    let cmd = match fs {
        Filesystem::Jffs2 => format!("flash_erase -j {} {} 0 0", options, target),
        Filesystem::Ext2 | Filesystem::Ext3 | Filesystem::Ext4 => {
            format!("mkfs.{} -F {} {}", fs, options, target)
        }
        Filesystem::Ubifs => format!("mkfs.{} -y {} {}", fs, options, target),
        Filesystem::Xfs => format!("mkfs.{} -f {} {}", fs, options, target),
        Filesystem::Btrfs | Filesystem::Vfat | Filesystem::F2fs => {
            format!("mkfs.{} {} {}", fs, options, target)
        }
    };

    easy_process::run(&cmd)?;
    Ok(())
}

pub(crate) fn mount_map<F, T>(source: &Path, fs: Filesystem, options: &str, f: F) -> Result<T>
where
    F: FnOnce(&Path) -> T,
{
    let tmpdir = tempfile::tempdir()?;
    let tmpdir = tmpdir.path();

    // We need to keep a guard otherwise it is dropped before the
    // closure is run.
    let _guard = mount(source, tmpdir, fs, options)?;

    Ok(f(tmpdir))
}

pub(crate) async fn mount_map_async<Fun, Fut, T>(
    source: &Path,
    fs: Filesystem,
    options: &str,
    f: Fun,
) -> Result<T>
where
    Fun: FnOnce(PathBuf) -> Fut,
    Fut: std::future::Future<Output = T>,
{
    let tmpdir = tempfile::tempdir()?;
    let tmpdir = tmpdir.path();

    // We need to keep a guard otherwise it is dropped before the
    // closure is run.
    let _guard = mount(source, tmpdir, fs, options)?;

    Ok(f(tmpdir.to_owned()).await)
}

pub(crate) fn mount(
    source: &Path,
    dest: &Path,
    fs: Filesystem,
    options: &str,
) -> io::Result<UnmountDrop<Mount>> {
    trace!("mounting {:?} as {} at {:?}", source, fs, dest);
    Ok(Mount::new(
        source,
        dest,
        format!("{}", fs).as_str(),
        sys_mount::MountFlags::empty(),
        Some(options),
    )?
    .into_unmount_drop(sys_mount::UnmountFlags::DETACH))
}

pub(crate) fn chmod(path: &Path, mode: u32) -> Result<()> {
    trace!("applying 0o{:o} permissions to {:?}", mode, path);
    nix::sys::stat::fchmodat(
        None,
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
