// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Error, Result};
use std::{
    fs,
    io::{BufRead, BufReader},
    path::PathBuf,
};

pub(crate) use ffi::is_nand;

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
        .filter_map(std::result::Result::ok)
        .find_map(|line| {
            re.captures(&line).and_then(|re_match| {
                let re_dev = re_match.name("dev").unwrap().as_str();
                let re_name = re_match.name("name").unwrap().as_str();
                if re_name == name { Some(PathBuf::from(format!("/dev/{}", re_dev))) } else { None }
            })
        })
        .ok_or_else(|| Error::NoMtdDevice(name.to_owned()))
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
        pub(crate) mtd: FakeMtd,
    }

    impl FakeUbi {
        pub(crate) fn new(names: &[&str], kind: MtdKind) -> Result<FakeUbi> {
            let mtd = FakeMtd::new(&["system"], kind)?;
            easy_process::run("modprobe ubi mtd=0")?;

            // Ubi created here so if anything fails the Drop will still be executed
            let ubi = FakeUbi { mtd };

            for name in names {
                easy_process::run(&format!("ubimkvol /dev/ubi0 -N {} -s 1MiB", name))?;
            }

            Ok(ubi)
        }
    }

    impl Drop for FakeUbi {
        fn drop(&mut self) {
            if let Err(e) = easy_process::run(&format!("rmmod ubi")) {
                eprintln!("Failed to cleanup FakeUbi, Error: {}", e);
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
            names.iter().enumerate().try_for_each(|(i, name)| {
                easy_process::run(&format!("mtdpart add /dev/mtd0 {} {} {}", name, i * 100, 100))
                    .map(|_| mtd.devices.push(PathBuf::from(format!("/dev/mtd{}", i + 1))))
            })?;

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

            if let Err(e) = easy_process::run(&format!("rmmod {}", module)) {
                eprintln!("Failed to cleanup FakeMtd, Error: {}", e);
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
            assert_eq!(is_nand(&PathBuf::from("/dev/mtd0")).unwrap(), true);
        }
        {
            let _mtd = FakeMtd::new(&[], MtdKind::Nor).unwrap();
            assert_eq!(is_nand(&PathBuf::from("/dev/mtd0")).unwrap(), false);
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
