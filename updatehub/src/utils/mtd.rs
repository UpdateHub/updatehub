// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

pub(crate) use self::ffi::is_nand;
use super::{Error, Result};
use std::{
    fs,
    io::{BufRead, BufReader},
    path::{Path, PathBuf},
};

pub(crate) fn target_device_from_ubi_volume_name(volume: &str) -> Result<PathBuf> {
    let re = regex::Regex::new(r"^Volume ID:   (?P<volume>\d+) \(on ubi(\d+)\)$").unwrap();
    walkdir::WalkDir::new("/dev")
        .min_depth(1)
        .into_iter()
        .filter_entry(|p| {
            p.file_name()
                .to_str()
                .map(|n| n.starts_with("ubi") && !n.contains('_'))
                .unwrap_or(false)
        })
        .filter_map(std::result::Result::ok)
        .find_map(|entry| {
            let path = entry.path();
            let output =
                easy_process::run(&format!("ubinfo {} -N {}", path.display(), volume)).ok()?;

            let line = output.stdout.lines().next()?;
            let re_match = re.captures(line)?;

            Some(PathBuf::from(format!(
                "{}_{}",
                path.display(),
                &re_match.name("volume").unwrap().as_str()
            )))
        })
        .ok_or_else(|| Error::NoUbiVolume(volume.to_owned()))
}

pub(crate) fn target_device_from_mtd_name(name: &str) -> Result<PathBuf> {
    let re =
        regex::Regex::new(r#"^(?P<dev>mtd\d): ([[:xdigit:]]+) ([[:xdigit:]]+) "(?P<name>.*)"$"#)
            .unwrap();
    let proc = fs::File::open("/proc/mtd")?;

    BufReader::new(proc)
        .lines()
        .map_while(std::result::Result::ok)
        .find_map(|line| {
            re.captures(&line).and_then(|re_match| {
                let re_dev = re_match.name("dev").unwrap().as_str();
                let re_name = re_match.name("name").unwrap().as_str();
                if re_name == name { Some(PathBuf::from(format!("/dev/{re_dev}"))) } else { None }
            })
        })
        .ok_or_else(|| Error::NoMtdDevice(name.to_owned()))
}

/// Mount sources which would be taken down by writing to `device`, an MTD
/// character device or a UBI volume.
///
/// Neither UBIFS nor JFFS2 mount a block device, so `/proc/self/mountinfo`
/// carries a name such as `ubi0:rootfs` or `mtd:rootfs` and the relationship
/// can only be reconstructed from it.
pub(crate) fn mount_sources_for(device: &Path) -> Vec<String> {
    let Some(name) = device.file_name().and_then(|name| name.to_str()) else {
        return Vec::default();
    };

    if name.starts_with("ubi") && name.contains('_') {
        return ubi_volume_sources(name);
    }

    match name.strip_prefix("mtd").map(str::parse) {
        Some(Ok(number)) => mtd_sources(number),
        _ => Vec::default(),
    }
}

/// Sources for a UBI volume, as in `ubi0_1`.
fn ubi_volume_sources(volume: &str) -> Vec<String> {
    let mut sources = vec![format!("/dev/{volume}"), volume.to_owned()];

    // UBIFS is usually mounted through the volume name, as in `ubi0:rootfs`.
    if let Some((device, _)) = volume.split_once('_') {
        if let Ok(name) = fs::read_to_string(format!("/sys/class/ubi/{volume}/name")) {
            sources.push(format!("{device}:{}", name.trim()));
        }
    }

    sources
}

fn mtd_sources(number: u32) -> Vec<String> {
    // mountinfo records the source as it was passed to mount(2), so the bare
    // names are kept alongside the absolute ones to cover a relative mount.
    let mut sources = vec![
        format!("/dev/mtd{number}"),
        format!("mtd{number}"),
        format!("/dev/mtdblock{number}"),
        format!("mtdblock{number}"),
    ];

    if let Ok(name) = fs::read_to_string(format!("/sys/class/mtd/mtd{number}/name")) {
        sources.push(format!("mtd:{}", name.trim()));
    }

    // A UBI device attached to this MTD keeps its volumes on it, so erasing the
    // MTD takes every mounted volume down as well.
    sources.extend(ubi_volumes_on_mtd(number).iter().flat_map(|volume| ubi_volume_sources(volume)));

    sources
}

/// Names of the UBI volumes, as in `ubi0_1`, stored on the given MTD device.
fn ubi_volumes_on_mtd(number: u32) -> Vec<String> {
    let Ok(entries) = fs::read_dir("/sys/class/ubi") else {
        return Vec::default();
    };

    // `/sys/class/ubi` lists both the UBI devices, `ubi0`, and their volumes,
    // `ubi0_1`, so the device a volume sits on is read off its own name.
    entries
        .filter_map(std::result::Result::ok)
        .map(|entry| entry.file_name().to_string_lossy().into_owned())
        .filter(|volume| {
            volume.split_once('_').is_some_and(|(device, _)| {
                fs::read_to_string(format!("/sys/class/ubi/{device}/mtd_num"))
                    .is_ok_and(|mtd| mtd.trim().parse() == Ok(number))
            })
        })
        .collect()
}

mod ffi {
    use crate::utils::Result;
    use nix::ioctl_read;
    use std::{mem::MaybeUninit, os::unix::io::AsRawFd, path::Path};

    // From https://github.com/torvalds/linux/blob/master/include/uapi/mtd/mtd-abi.h
    const MTD_NANDFLASH: u8 = 4;
    const MTD_MLCNANDFLASH: u8 = 8;
    const MEMGETINFO: u8 = b'M';
    const MEMGETINFO_MODE: u8 = 1;

    #[repr(C)]
    pub struct mtd_info_user {
        kind: u8,
        flags: u32,
        size: u32,
        erasesize: u32,
        writesize: u32,
        oobsize: u32,
        padding: u64,
    }

    ioctl_read!(mtd_get_info, MEMGETINFO, MEMGETINFO_MODE, mtd_info_user);

    pub fn is_nand(device: &Path) -> Result<bool> {
        let device = std::fs::File::open(device)?;
        let info = unsafe {
            let mut info = MaybeUninit::<mtd_info_user>::uninit();
            mtd_get_info(device.as_raw_fd(), info.as_mut_ptr())?;
            info.assume_init()
        };

        Ok(info.kind == MTD_NANDFLASH || info.kind == MTD_MLCNANDFLASH)
    }
}

#[cfg(test)]
pub(crate) mod tests {
    use super::*;
    use lazy_static::lazy_static;
    use pretty_assertions::assert_eq;
    use std::sync::{Arc, Mutex};

    pub(crate) struct FakeUbi {
        #[allow(dead_code)]
        pub(crate) mtd_guard: FakeMtd,
    }

    impl FakeUbi {
        pub(crate) fn new(names: &[&str], kind: MtdKind) -> Result<FakeUbi> {
            let mtd_guard = FakeMtd::new(&["system"], kind)?;
            easy_process::run("modprobe ubi mtd=0")?;

            // Ubi created here so if anything fails the Drop will still be executed
            let ubi = FakeUbi { mtd_guard };

            for name in names {
                easy_process::run(&format!("ubimkvol /dev/ubi0 -N {name} -s 1MiB"))?;
            }

            Ok(ubi)
        }
    }

    impl Drop for FakeUbi {
        fn drop(&mut self) {
            if let Err(e) = easy_process::run("rmmod ubi") {
                eprintln!("Failed to cleanup FakeUbi, Error: {e}");
            }
        }
    }

    pub(crate) struct FakeMtd {
        pub(crate) devices: Vec<PathBuf>,
        pub(crate) kind: MtdKind,
    }

    pub(crate) enum MtdKind {
        Nand,
        Nor,
    }

    impl FakeMtd {
        pub(crate) fn new(names: &[&str], kind: MtdKind) -> Result<FakeMtd> {
            match kind {
                MtdKind::Nand => easy_process::run("modprobe nandsim second_id_byte=0x36"),
                MtdKind::Nor => easy_process::run("modprobe mtdram total_size=20000"),
            }?;

            // FakeMtd created here so if any subsequent command fails the drop will still
            // be called to cleanup mtd devices
            let mut mtd = FakeMtd { devices: vec![], kind };

            // mtdpart wants the offset and the size aligned on the erase block,
            // and a partition below the object size fails `ensure_disk_space`.
            let erase_size = fs::read_to_string("/sys/class/mtd/mtd0/erasesize")?
                .trim()
                .parse::<usize>()
                .map_err(|e| std::io::Error::new(std::io::ErrorKind::InvalidData, e))?;

            for (i, name) in names.iter().enumerate() {
                easy_process::run(&format!(
                    "mtdpart add /dev/mtd0 {} {} {}",
                    name,
                    i * erase_size,
                    erase_size
                ))?;
                mtd.devices.push(PathBuf::from(format!("/dev/mtd{}", i + 1)));
            }

            Ok(mtd)
        }
    }

    impl Drop for FakeMtd {
        fn drop(&mut self) {
            let module = match self.kind {
                MtdKind::Nand => "nandsim",
                MtdKind::Nor => "mtdram",
            };

            // Sleep time for nandsim to sync and avoid errors
            std::thread::sleep(std::time::Duration::from_millis(500));

            if let Err(e) = easy_process::run(&format!("rmmod {module}")) {
                eprintln!("Failed to cleanup FakeMtd, Error: {e}");
            }
        }
    }

    // Used to serialize access to MTD devices
    lazy_static! {
        pub static ref SERIALIZE: Arc<Mutex<()>> = Arc::new(Mutex::default());
    }

    #[test]
    #[ignore]
    fn device_from_mtd_name() {
        let _lock = SERIALIZE.lock();
        let dev_names = vec!["system0", "system1"];

        let mtd = FakeMtd::new(&dev_names, MtdKind::Nor).unwrap();

        assert_eq!(
            dev_names
                .into_iter()
                .map(target_device_from_mtd_name)
                .map(Result::unwrap)
                .collect::<Vec<_>>(),
            mtd.devices,
        );
        assert!(target_device_from_mtd_name("some_inexistent_device").is_err());
    }

    #[test]
    #[ignore]
    fn test_is_nand() {
        let _lock = SERIALIZE.lock();

        {
            let _mtd = FakeMtd::new(&[], MtdKind::Nand).unwrap();
            assert!(is_nand(&PathBuf::from("/dev/mtd0")).unwrap());
        }
        {
            let _mtd = FakeMtd::new(&[], MtdKind::Nor).unwrap();
            assert!(!is_nand(&PathBuf::from("/dev/mtd0")).unwrap());
        }
    }

    #[test]
    #[ignore]
    fn device_from_ubi_volume_name() {
        let _lock = SERIALIZE.lock();
        let volume_names = vec!["some_ui_volume", "another_ubi_volume"];

        let _ubi = FakeUbi::new(&volume_names, MtdKind::Nor).unwrap();
        assert_eq!(
            target_device_from_ubi_volume_name(volume_names[1]).unwrap(),
            PathBuf::from("/dev/ubi0_1")
        );
        assert_eq!(
            target_device_from_ubi_volume_name(volume_names[0]).unwrap(),
            PathBuf::from("/dev/ubi0_0")
        );
    }

    #[test]
    #[ignore]
    fn device_from_ubi_volume_name_multiple_volumes() {
        let _lock = SERIALIZE.lock();
        let volume_names = vec![
            "volume0", "volume1", "volume2", "volume3", "volume4", "volume5", "volume6", "volume7",
            "volume8", "volume9", "volume10", "volume11", "volume12", "volume13",
        ];

        let _ubi = FakeUbi::new(&volume_names, MtdKind::Nor).unwrap();
        assert_eq!(
            target_device_from_ubi_volume_name(volume_names[8]).unwrap(),
            PathBuf::from("/dev/ubi0_8")
        );
        assert_eq!(
            target_device_from_ubi_volume_name(volume_names[12]).unwrap(),
            PathBuf::from("/dev/ubi0_12")
        );
    }
}
