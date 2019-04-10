// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use crate::update_package::object::definitions::{
    target_permissions::{Gid, Uid},
    Filesystem,
};
use easy_process;
use std::{io, path::Path};
use sys_mount::{Mount, Unmount, UnmountDrop};

pub(crate) fn format(
    target: &Path,
    fs: Filesystem,
    options: &Option<String>,
) -> Result<(), failure::Error> {
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

pub(crate) fn mount_map<F>(
    source: &Path,
    fs: Filesystem,
    options: &str,
    f: F,
) -> Result<(), failure::Error>
where
    F: FnOnce(&Path) -> Result<(), failure::Error>,
{
    let tmpdir = tempfile::tempdir()?;
    let tmpdir = tmpdir.path();

    // We need to keep a guard otherwise it is dropped before the
    // closure is run.
    let _guard = mount(source, &tmpdir, fs, options)?;

    f(tmpdir)
}

pub(crate) fn mount(
    source: &Path,
    dest: &Path,
    fs: Filesystem,
    options: &str,
) -> io::Result<UnmountDrop<Mount>> {
    Ok(Mount::new(
        source,
        dest,
        format!("{}", fs).as_str(),
        sys_mount::MountFlags::empty(),
        Some(options),
    )?
    .into_unmount_drop(sys_mount::UnmountFlags::DETACH))
}

pub(crate) fn chmod(path: &Path, mode: u32) -> Result<(), failure::Error> {
    nix::sys::stat::fchmodat(
        None,
        path,
        nix::sys::stat::Mode::from_bits(mode).unwrap(),
        nix::sys::stat::FchmodatFlags::FollowSymlink,
    )?;

    Ok(())
}

pub(crate) fn chown(path: &Path, uid: &Option<Uid>, gid: &Option<Gid>) -> nix::Result<()> {
    nix::unistd::chown(
        path,
        uid.as_ref()
            .map(|id| nix::unistd::Uid::from_raw(id.as_u32())),
        gid.as_ref()
            .map(|id| nix::unistd::Gid::from_raw(id.as_u32())),
    )
}
