// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Deserialize;
use std::fmt;

/// Filesystem type that must be used to mount device
#[derive(Deserialize, PartialEq, Debug, Copy, Clone)]
#[serde(rename_all = "lowercase")]
pub enum Filesystem {
    Btrfs,
    Ext2,
    Ext3,
    Ext4,
    Vfat,
    F2fs,
    Jffs2,
    Ubifs,
    Xfs,
}

impl fmt::Display for Filesystem {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        fmt::Display::fmt(
            match self {
                Filesystem::Btrfs => "btrfs",
                Filesystem::Ext2 => "ext2",
                Filesystem::Ext3 => "ext3",
                Filesystem::Ext4 => "ext4",
                Filesystem::Vfat => "vfat",
                Filesystem::F2fs => "f2fs",
                Filesystem::Jffs2 => "jffs2",
                Filesystem::Ubifs => "ubifs",
                Filesystem::Xfs => "xfs",
            },
            f,
        )
    }
}
