// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use failure::format_err;
use std::{
    fs,
    io::{BufRead, BufReader},
    path::PathBuf,
};

pub(crate) use ffi::is_nand;

pub(crate) fn target_device_from_ubi_volume_name(volume: &str) -> Result<PathBuf, failure::Error> {
    let re = regex::Regex::new(r"^Volume ID:   (\d) \(on ubi(\d)\)$").unwrap();
    let path = fs::read_dir("/dev")?
        .filter(|entry| entry.is_ok())
        .map(|entry| format!("{:?}", entry.unwrap().path()))
        .find(|path| path.starts_with("ubi"))
        .ok_or_else(|| format_err!("Unable to find coorespoing ubi volume"))?;

    let dev_number = path.replace("ubi", "");

    let output = easy_process::run(&format!("ubinfo -d {} -N {}", dev_number, volume))?;
    let line = output
        .stdout
        .lines()
        .next()
        .ok_or_else(|| format_err!("Unable to read first line of ubinfo"))?;

    let re_match = re
        .captures(line)
        .ok_or_else(|| format_err!("Unable to extract any matches for Volume ID"))?;

    Ok(PathBuf::from(format!(
        "/dev/ubi{}_{}",
        dev_number, &re_match[0]
    )))
}

pub(crate) fn target_device_from_mtd_name(name: &str) -> Result<PathBuf, failure::Error> {
    let re =
        regex::Regex::new(r#"^(?P<dev>mtd\d): ([[:xdigit:]]+) ([[:xdigit:]]+) "(?P<name>.*)"$"#)
            .unwrap();
    let proc = fs::File::open("/proc/mtd")?;

    BufReader::new(proc)
        .lines()
        .filter_map(Result::ok)
        .find_map(|line| {
            re.captures(&line).and_then(|re_match| {
                let re_dev = re_match.name("dev").unwrap().as_str();
                let re_name = re_match.name("name").unwrap().as_str();
                if re_name == name {
                    Some(PathBuf::from(format!("/dev/{}", re_dev)))
                } else {
                    None
                }
            })
        })
        .ok_or_else(|| format_err!("Unable to find match for mtd device: {}", name))
}

mod ffi {
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

    pub fn is_nand(device: &Path) -> Result<bool, failure::Error> {
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

    pub(crate) struct FakeMtd {
        pub(crate) devices: Vec<PathBuf>,
        pub(crate) kind: MtdKind,
    }

    pub(crate) enum MtdKind {
        Nand,
        Nor,
    }

    impl FakeMtd {
        pub(crate) fn new(names: &[&str], kind: MtdKind) -> Result<FakeMtd, failure::Error> {
            match kind {
                MtdKind::Nand => {
                    easy_process::run("modprobe nandsim second_id_byte=0x36")?;
                    easy_process::run("mtdpart del /dev/mtd0 1")?;
                }
                MtdKind::Nor => {
                    easy_process::run("modprobe mtdram total_size=1000 erase_size=10")?;
                }
            }
            let total_size = 1000;

            // FakeMtd created here so if any subsequent command fails the drop will still
            // be called to cleanup mtd devices
            let mut mtd = FakeMtd {
                devices: vec![],
                kind,
            };
            let size = total_size / names.len();
            names.iter().enumerate().try_for_each(|(i, n)| {
                easy_process::run(&format!(
                    "mtdpart add /dev/mtd0 {} {} {}",
                    n,
                    i * size,
                    size
                ))
                .map(|_| {
                    mtd.devices
                        .push(PathBuf::from(format!("/dev/mtd{}", i + 1)))
                })
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
    fn device_from_mtd_name_nor() -> Result<(), failure::Error> {
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

        Ok(())
    }

    #[test]
    #[ignore]
    fn device_from_mtd_name_nand() -> Result<(), failure::Error> {
        let _lock = SERIALIZE.lock();
        let dev_names = vec!["system0", "system1"];

        let mtd = FakeMtd::new(&dev_names, MtdKind::Nand).unwrap();

        assert_eq!(
            dev_names
                .into_iter()
                .map(target_device_from_mtd_name)
                .map(Result::unwrap)
                .collect::<Vec<_>>(),
            mtd.devices,
        );
        assert!(target_device_from_mtd_name("some_inexistent_device").is_err());

        Ok(())
    }

    #[test]
    #[ignore]
    fn test_is_nand() -> Result<(), failure::Error> {
        let _lock = SERIALIZE.lock();
        let dev_names = vec!["system0"];

        {
            let mtd = FakeMtd::new(&dev_names, MtdKind::Nand).unwrap();
            assert_eq!(is_nand(&mtd.devices[0]).unwrap(), true);
        }
        {
            let mtd = FakeMtd::new(&dev_names, MtdKind::Nor).unwrap();
            assert_eq!(is_nand(&mtd.devices[0]).unwrap(), false);
        }

        Ok(())
    }
}
