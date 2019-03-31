// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::Deserialize;

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
