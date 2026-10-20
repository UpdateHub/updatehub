// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Context, Error, Result};
use crate::{
    object::{Info, Installer},
    utils::{
        self,
        definitions::{Access, TargetTypeExt},
        log::LogContent,
    },
};
use pkg_schema::{definitions, objects};
use slog_scope::info;
use std::{io::SeekFrom, path::Path};
use tokio::{
    fs,
    io::{AsyncRead, AsyncReadExt, AsyncSeekExt},
};
use tokio_take_seek::AsyncTakeSeekExt;

impl Installer for objects::Raw {
    async fn check_requirements(&self, _: &Context) -> Result<()> {
        info!("'raw' handle checking requirements");

        let access = checked_region(self)
            .map_or(Access::Exclusive, |(offset, len)| Access::Region { offset, len });
        if let definitions::TargetType::Device(dev) =
            self.target_type.valid(access).log_error_msg("device failed vaidation")?
        {
            utils::fs::ensure_disk_space(dev, self.required_install_size())
                .log_error_msg("not enough disk space")?;
            return Ok(());
        }

        Err(Error::InvalidTargetType(self.target_type.clone()))
    }

    async fn install(&self, context: &Context) -> Result<()> {
        info!("'raw' handler Install {} ({})", self.filename, self.sha256sum);

        let definitions::TargetType::Device(ref device) = self.target_type else {
            unreachable!("device should be secured by check_requirements")
        };
        let source = context.download_dir.join(self.sha256sum());
        let chunk_size = self.chunk_size.0;
        let truncate = self.truncate.0;
        // The check must stop at the end of what this object writes, otherwise
        // a neighbouring object sharing the device answers for it.
        let (seek, region_len) = target_region(self);

        let should_skip_install = super::should_skip_install(
            self.install_if_different.as_ref(),
            &self.sha256sum,
            async {
                let h = fs::OpenOptions::new().read(true).open(device).await?;
                let mut h = utils::io::timed_buf_reader(chunk_size, h);
                h.seek(SeekFrom::Start(seek)).await?;
                Ok(h.take_with_seek(region_len))
            },
        )
        .await?;
        if should_skip_install {
            return Ok(());
        }

        // The region check lets a whole disk be written beside the partitions
        // it does not reach, even when they are mounted. Only then does the
        // region matter, so only then is the write held to it, and the data
        // is checked to fit before the device is touched.
        let limit = match checked_region(self) {
            Some((_, len)) if utils::fs::ensure_not_mounted(device).is_err() => {
                ensure_fits(self, &source, len).await?;
                Some(len)
            }
            _ => None,
        };

        let mut input = open_source(self, &source).await?;
        let mut target = {
            let mut target = utils::io::timed_buf_writer(
                chunk_size,
                fs::OpenOptions::new()
                    .write(true)
                    .truncate(truncate)
                    .open(device)
                    .await
                    .log_error_msg("failed to open target file")?,
            );
            target.seek(SeekFrom::Start(seek)).await.log_error_msg("failed to seek target file")?;
            utils::io::BoundedWriter::new(target, limit.unwrap_or(u64::MAX))
        };

        if self.compressed {
            compress_tools::tokio_support::uncompress_data(&mut input, &mut target)
                .await
                .log_error_msg("failed to uncompress data")?;
        } else {
            tokio::io::copy(&mut input, &mut target)
                .await
                .log_error_msg("failed copy from source into target")?;
        }

        Ok(())
    }
}

async fn open_source(obj: &objects::Raw, source: &Path) -> Result<Box<dyn AsyncRead + Unpin>> {
    let mut input = utils::io::timed_buf_reader(
        obj.chunk_size.0,
        fs::File::open(source).await.log_error_msg("failed to open source file")?,
    );
    input
        .seek(SeekFrom::Start(obj.skip.0 * obj.chunk_size.0 as u64))
        .await
        .log_error_msg("failed to seek source file")?;

    Ok(match count_bytes(obj) {
        None => Box::new(input),
        Some(n) => Box::new(input.take(n)),
    })
}

/// A compressed source is uncompressed to measure it, only its metadata
/// telling its size otherwise.
async fn ensure_fits(obj: &objects::Raw, source: &Path, len: u64) -> Result<()> {
    let fits = if obj.compressed {
        let mut input = open_source(obj, source).await?;
        let mut sink = utils::io::BoundedWriter::new(tokio::io::sink(), len);
        let res = compress_tools::tokio_support::uncompress_data(&mut input, &mut sink).await;
        // Going past the limit fails the uncompression too, that is not a
        // corrupted source.
        if !sink.overflowed() {
            res.log_error_msg("failed to uncompress data")?;
        }
        !sink.overflowed()
    } else {
        let available = fs::metadata(source)
            .await
            .log_error_msg("failed to read source file metadata")?
            .len()
            .saturating_sub(obj.skip.0 * obj.chunk_size.0 as u64);
        count_bytes(obj).map_or(available, |n| n.min(available)) <= len
    };

    if !fits {
        return Err(Error::LargerThanDeclared(obj.filename.clone()))
            .log_error_msg("object does not fit the region checked for it");
    }

    Ok(())
}

fn count_bytes(obj: &objects::Raw) -> Option<u64> {
    match obj.count {
        definitions::Count::All => None,
        definitions::Count::Limited(n) => {
            Some(u64::try_from(n).unwrap_or(0) * obj.chunk_size.0 as u64)
        }
    }
}

/// Offset and length, in bytes, of what `obj` writes to its target.
fn target_region(obj: &objects::Raw) -> (u64, u64) {
    let chunk_size = obj.chunk_size.0 as u64;
    let len = if obj.compressed {
        obj.required_uncompressed_size
    } else {
        let written = obj.size.saturating_sub(obj.skip.0 * chunk_size);
        count_bytes(obj).map_or(written, |n| n.min(written))
    };

    (obj.seek * chunk_size, len)
}

/// The region requirements are checked for, when it is known up front. A
/// compressed object not declaring its uncompressed size is not.
fn checked_region(obj: &objects::Raw) -> Option<(u64, u64)> {
    (!obj.compressed || obj.required_uncompressed_size != 0).then(|| target_region(obj))
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::utils::fs::tests::{FakeLoopDevice, LOOP_DEVICE_SIZE};
    use flate2::{Compression, write::GzEncoder};
    use pretty_assertions::assert_eq;
    use std::io::{Seek, Write};
    use tempfile::{NamedTempFile, TempDir, tempdir};
    use tokio::io::{self, AsyncBufReadExt};

    const DEFAULT_BYTE: u8 = 0xF;
    const ORIGINAL_BYTE: u8 = 0xA;

    fn fake_raw_object(
        size: u64,
        chunk_size: usize,
        skip: u64,
        seek: u64,
        count: definitions::Count,
        truncate: bool,
        compressed: bool,
    ) -> Result<(objects::Raw, TempDir, NamedTempFile, NamedTempFile, Vec<u8>)> {
        let download_dir = tempdir()?;

        let mut source = NamedTempFile::new_in(download_dir.path())?;
        let data_len = usize::try_from(size).unwrap();
        let original_data = std::iter::repeat_n(ORIGINAL_BYTE, data_len).collect::<Vec<_>>();
        let data = if compressed {
            let mut e = GzEncoder::new(Vec::new(), Compression::default());
            e.write_all(&original_data).unwrap();
            e.finish().unwrap()
        } else {
            original_data.clone()
        };
        source.write_all(&data)?;
        source.seek(SeekFrom::Start(0))?;

        let mut dest = NamedTempFile::new_in(download_dir.path())?;
        dest.write_all(&std::iter::repeat_n(DEFAULT_BYTE, data_len).collect::<Vec<_>>())?;
        dest.seek(SeekFrom::Start(0))?;

        Ok((
            objects::Raw {
                filename: String::new(),
                size,
                sha256sum: source.path().to_string_lossy().to_string(),
                target_type: definitions::TargetType::Device(dest.path().into()),

                install_if_different: None,
                compressed,
                required_uncompressed_size: 0,
                chunk_size: definitions::ChunkSize(chunk_size),
                skip: definitions::Skip(skip),
                seek,
                count,
                truncate: definitions::Truncate(truncate),
            },
            download_dir,
            source,
            dest,
            original_data,
        ))
    }

    async fn check_unwritten_blocks(
        f: &std::path::Path,
        offset: u64,
        byte_count: u64,
    ) -> io::Result<()> {
        let f = fs::File::open(f).await?;
        let mut f = io::BufReader::with_capacity(1, f);
        f.seek(SeekFrom::Start(offset)).await?;
        for i in 0..byte_count {
            let buf = f.fill_buf().await?;
            let len = buf.len();

            assert_eq!(buf, [DEFAULT_BYTE], "Error on byte {}", i);

            f.consume(len);
        }
        Ok(())
    }

    async fn validate_file(
        data: Vec<u8>,
        file: &std::path::Path,
        chunk_size: usize,
        skip: u64,
        seek: u64,
        count: definitions::Count,
    ) -> io::Result<()> {
        let skip = usize::try_from(skip).unwrap() * chunk_size;
        let file = fs::File::open(file).await?;
        let mut f1 = io::BufReader::with_capacity(chunk_size, &data[skip..]);
        let mut f2 = io::BufReader::with_capacity(chunk_size, file);
        f2.seek(SeekFrom::Start(seek * chunk_size as u64)).await?;

        for _ in count {
            let buf1 = f1.fill_buf().await?;
            let len1 = buf1.len();
            let buf2 = f2.fill_buf().await?;
            let len2 = buf2.len();

            // Stop comparing if either the files reach EOF
            if len1 == 0 || len2 == 0 {
                break;
            }

            assert_eq!(buf1, buf2);
            f1.consume(len1);
            f2.consume(len2);
        }
        Ok(())
    }

    const SHARED_CHUNK_SIZE: usize = 1024;
    const SHARED_OBJECT_LEN: usize = 4 * SHARED_CHUNK_SIZE;
    const SHARED_OBJECT_AT: usize = SHARED_CHUNK_SIZE;
    const SHARED_NEIGHBOUR_AT: usize = 64 * SHARED_CHUNK_SIZE;

    /// Lays out a device shared by two raw objects, the one under test at
    /// `SHARED_OBJECT_AT` and a neighbour at `SHARED_NEIGHBOUR_AT`, each
    /// holding a U-Boot banner of the given version.
    fn fake_shared_device(
        rule_version: &str,
        object_version: Option<&str>,
        neighbour_version: &str,
    ) -> (objects::Raw, TempDir, NamedTempFile, NamedTempFile) {
        let banner = |v: &str| format!("U-Boot {v} (Jul 18 2023 - 18:41:01 +0000)\0").into_bytes();

        let download_dir = tempdir().unwrap();

        let mut source = NamedTempFile::new_in(download_dir.path()).unwrap();
        source
            .write_all(&std::iter::repeat_n(ORIGINAL_BYTE, SHARED_OBJECT_LEN).collect::<Vec<_>>())
            .unwrap();
        source.flush().unwrap();

        let mut dest = NamedTempFile::new_in(download_dir.path()).unwrap();
        dest.write_all(
            &std::iter::repeat_n(DEFAULT_BYTE, 2 * SHARED_NEIGHBOUR_AT).collect::<Vec<_>>(),
        )
        .unwrap();
        if let Some(v) = object_version {
            dest.seek(SeekFrom::Start(SHARED_OBJECT_AT as u64)).unwrap();
            dest.write_all(&banner(v)).unwrap();
        }
        dest.seek(SeekFrom::Start(SHARED_NEIGHBOUR_AT as u64)).unwrap();
        dest.write_all(&banner(neighbour_version)).unwrap();
        dest.flush().unwrap();

        let obj = objects::Raw {
            filename: String::new(),
            size: SHARED_OBJECT_LEN as u64,
            sha256sum: source.path().to_string_lossy().to_string(),
            target_type: definitions::TargetType::Device(dest.path().into()),

            install_if_different: Some(definitions::InstallIfDifferent::KnownPattern {
                version: rule_version.to_string(),
                pattern: definitions::install_if_different::KnownPatternKind::UBoot,
            }),
            compressed: false,
            required_uncompressed_size: 0,
            chunk_size: definitions::ChunkSize(SHARED_CHUNK_SIZE),
            skip: definitions::Skip(0),
            seek: (SHARED_OBJECT_AT / SHARED_CHUNK_SIZE) as u64,
            count: definitions::Count::All,
            truncate: definitions::Truncate(false),
        };

        (obj, download_dir, source, dest)
    }

    #[test]
    fn region_covers_what_is_written() -> Result<()> {
        let (obj, ..) =
            fake_raw_object(8192, 1024, 1, 4, definitions::Count::Limited(2), false, false)?;
        assert_eq!(target_region(&obj), (4096, 2048));

        let (obj, ..) = fake_raw_object(8192, 1024, 1, 4, definitions::Count::All, false, false)?;
        assert_eq!(target_region(&obj), (4096, 7168));

        let (obj, ..) =
            fake_raw_object(8192, 1024, 0, 0, definitions::Count::Limited(16), false, false)?;
        assert_eq!(target_region(&obj), (0, 8192));

        let (mut obj, ..) =
            fake_raw_object(8192, 1024, 0, 2, definitions::Count::All, false, true)?;
        obj.required_uncompressed_size = 8192;
        assert_eq!(target_region(&obj), (2048, 8192));

        Ok(())
    }

    #[test]
    fn compressed_object_without_its_size_has_no_checked_region() -> Result<()> {
        let (mut obj, ..) =
            fake_raw_object(8192, 1024, 0, 2, definitions::Count::All, false, true)?;
        assert_eq!(checked_region(&obj), None);

        obj.required_uncompressed_size = 8192;
        assert_eq!(checked_region(&obj), Some((2048, 8192)));

        Ok(())
    }

    fn gzip(data: &[u8]) -> Vec<u8> {
        let mut e = GzEncoder::new(Vec::new(), Compression::default());
        e.write_all(data).unwrap();
        e.finish().unwrap()
    }

    fn raw_object(
        download_dir: &std::path::Path,
        name: &str,
        data: &[u8],
        compressed: bool,
        seek: u64,
        target: &std::path::Path,
    ) -> objects::Raw {
        let source = if compressed { gzip(data) } else { data.to_vec() };
        std::fs::write(download_dir.join(name), &source).unwrap();

        objects::Raw {
            filename: name.to_string(),
            size: source.len() as u64,
            sha256sum: name.to_string(),
            target_type: definitions::TargetType::Device(target.into()),

            install_if_different: None,
            compressed,
            required_uncompressed_size: if compressed { data.len() as u64 } else { 0 },
            chunk_size: definitions::ChunkSize(512),
            skip: definitions::Skip(0),
            seek: seek / 512,
            count: definitions::Count::All,
            truncate: definitions::Truncate(false),
        }
    }

    #[tokio::test]
    async fn source_fitting_its_region_fits() {
        let dir = tempdir().unwrap();
        let data = vec![ORIGINAL_BYTE; 4096];
        for compressed in [false, true] {
            let obj = raw_object(dir.path(), "obj", &data, compressed, 0, dir.path());
            let source = dir.path().join("obj");
            ensure_fits(&obj, &source, 4096).await.unwrap();
        }
    }

    #[tokio::test]
    async fn source_longer_than_its_region_does_not_fit() {
        let dir = tempdir().unwrap();
        let data = vec![ORIGINAL_BYTE; 4096];
        for compressed in [false, true] {
            let obj = raw_object(dir.path(), "obj", &data, compressed, 0, dir.path());
            let source = dir.path().join("obj");
            let res = ensure_fits(&obj, &source, 4095).await;
            assert!(
                matches!(res, Err(Error::LargerThanDeclared(_))),
                "compressed: {compressed}, got {res:?}"
            );
        }
    }

    #[tokio::test]
    async fn source_fits_once_cut_by_skip_and_count() {
        let dir = tempdir().unwrap();
        let mut obj = raw_object(dir.path(), "obj", &[ORIGINAL_BYTE; 4096], false, 0, dir.path());
        let source = dir.path().join("obj");
        obj.skip = definitions::Skip(2);
        ensure_fits(&obj, &source, 3072).await.unwrap();
        assert!(ensure_fits(&obj, &source, 3071).await.is_err());

        obj.count = definitions::Count::Limited(1);
        ensure_fits(&obj, &source, 512).await.unwrap();
    }

    #[tokio::test]
    async fn corrupted_compressed_source_is_not_reported_as_too_large() {
        let dir = tempdir().unwrap();
        let mut obj = raw_object(dir.path(), "obj", &[ORIGINAL_BYTE; 4096], true, 0, dir.path());
        let source = dir.path().join("obj");
        let mut gz = std::fs::read(&source).unwrap();
        gz.truncate(gz.len() / 2);
        std::fs::write(&source, gz).unwrap();
        obj.size /= 2;

        let res = ensure_fits(&obj, &source, 4096).await;
        assert!(matches!(res, Err(Error::Uncompress(_))), "got {res:?}");
    }

    /// Nothing else being in use, the region is irrelevant and the object is
    /// written whole, however much its metadata understates it.
    #[tokio::test]
    async fn source_longer_than_declared_is_written_whole_when_nothing_is_mounted() {
        for compressed in [false, true] {
            let (mut obj, download_dir, _source_guard, target_guard, _) =
                fake_raw_object(2048, 8, 0, 0, definitions::Count::All, false, compressed).unwrap();
            if compressed {
                obj.required_uncompressed_size = 1024;
            } else {
                obj.size = 1024;
            }
            let context =
                Context { download_dir: download_dir.path().to_owned(), ..Context::default() };

            obj.check_requirements(&context).await.unwrap();
            obj.install(&context).await.unwrap();

            let target = std::fs::read(target_guard.path()).unwrap();
            assert!(target.iter().all(|b| *b == ORIGINAL_BYTE), "compressed: {compressed}");
        }
    }

    /// Bootloader layout of an am335x: MLO at 128 KiB and U-Boot at 384 KiB of
    /// the disk, the rootfs mounted from 1 MiB.
    const PARTITION_START: u64 = 1024 * 1024;
    const MLO_AT: u64 = 0x2_0000;
    const UBOOT_AT: u64 = 0x6_0000;

    fn mounted_disk() -> (FakeLoopDevice, utils::fs::MountGuard) {
        let (disk, partition) =
            FakeLoopDevice::with_partition(LOOP_DEVICE_SIZE, PARTITION_START).unwrap();
        utils::fs::format(&partition, definitions::Filesystem::Ext4, None).unwrap();
        let guard = utils::fs::mount(&partition, definitions::Filesystem::Ext4, "").unwrap();
        std::fs::write(guard.mount_point().join("canary"), b"alive").unwrap();

        (disk, guard)
    }

    fn read_at(device: &std::path::Path, offset: u64, len: usize) -> Vec<u8> {
        use std::os::unix::fs::FileExt;

        let mut buf = vec![0; len];
        std::fs::File::open(device).unwrap().read_exact_at(&mut buf, offset).unwrap();
        buf
    }

    #[tokio::test]
    #[ignore = "needs root to attach a loop device"]
    async fn bootloader_installs_beside_a_mounted_partition() {
        let (disk, guard) = mounted_disk();
        let download_dir = tempdir().unwrap();
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };
        let rootfs_before = read_at(&disk.device, PARTITION_START, 64 * 1024);

        for compressed in [false, true] {
            let mlo = vec![0x5a; 0x1_415c];
            let uboot = vec![0xa5; 0x6_2f48];
            for (name, data, at) in [("mlo", &mlo, MLO_AT), ("uboot", &uboot, UBOOT_AT)] {
                let obj = raw_object(download_dir.path(), name, data, compressed, at, &disk.device);
                obj.check_requirements(&context).await.unwrap();
                obj.install(&context).await.unwrap();

                assert_eq!(&read_at(&disk.device, at, data.len()), data, "{name}, {compressed}");
            }
        }

        assert_eq!(read_at(&disk.device, PARTITION_START, 64 * 1024), rootfs_before);
        assert_eq!(std::fs::read(guard.mount_point().join("canary")).unwrap(), b"alive");
    }

    #[tokio::test]
    #[ignore = "needs root to attach a loop device"]
    async fn object_reaching_a_mounted_partition_is_refused() {
        let (disk, _guard) = mounted_disk();
        let download_dir = tempdir().unwrap();
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };

        // Ends a single sector into the partition.
        let data = vec![0x5a; usize::try_from(PARTITION_START - UBOOT_AT).unwrap() + 512];
        let obj = raw_object(download_dir.path(), "uboot", &data, false, UBOOT_AT, &disk.device);
        let res = obj.check_requirements(&context).await;

        assert!(matches!(res, Err(Error::Utils(utils::Error::DeviceInUse { .. }))), "got {res:?}");
    }

    /// Overwriting the partition table beside a mounted partition leaves the
    /// mount working, but the next boot finds no partition to mount.
    #[tokio::test]
    #[ignore = "needs root to attach a loop device"]
    async fn partition_table_beside_a_mounted_partition_is_refused() {
        let download_dir = tempdir().unwrap();
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };
        let sectors = LOOP_DEVICE_SIZE / 512;
        let mbr = FakeLoopDevice::with_partition(LOOP_DEVICE_SIZE, PARTITION_START).unwrap();
        let gpt = FakeLoopDevice::with_image(LOOP_DEVICE_SIZE, 512, &[1], |f| {
            utils::partition_table::tests::Gpt::new(512, sectors, vec![(2048, sectors - 34)])
                .write(f)
        })
        .map(|(disk, mut partitions)| (disk, partitions.remove(0)))
        .unwrap();

        for ((disk, partition), regions) in
            [(mbr, vec![(0, 512)]), (gpt, vec![(0, 34 * 512), ((sectors - 33) * 512, 33 * 512)])]
        {
            utils::fs::format(&partition, definitions::Filesystem::Ext4, None).unwrap();
            let _guard = utils::fs::mount(&partition, definitions::Filesystem::Ext4, "").unwrap();

            for (at, len) in regions {
                let data = vec![0; usize::try_from(len).unwrap()];
                let obj = raw_object(download_dir.path(), "table", &data, false, at, &disk.device);
                let res = obj.check_requirements(&context).await;

                assert!(
                    matches!(res, Err(Error::Utils(utils::Error::PartitionTableInUse { .. }))),
                    "{at:#x}+{len:#x}: got {res:?}"
                );
            }
        }
    }

    /// A BIOS loader such as GRUB's boot.img replaces the boot code ahead of
    /// the partition table.
    #[tokio::test]
    #[ignore = "needs root to attach a loop device"]
    async fn boot_code_installs_beside_a_mounted_partition() {
        let (disk, guard) = mounted_disk();
        let download_dir = tempdir().unwrap();
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };
        let table_before = read_at(&disk.device, 440, 72);

        let boot_code = vec![0x5a; 440];
        let obj = raw_object(download_dir.path(), "boot", &boot_code, false, 0, &disk.device);
        obj.check_requirements(&context).await.unwrap();
        obj.install(&context).await.unwrap();

        assert_eq!(read_at(&disk.device, 0, 440), boot_code);
        assert_eq!(read_at(&disk.device, 440, 72), table_before);
        assert_eq!(std::fs::read(guard.mount_point().join("canary")).unwrap(), b"alive");
    }

    /// Beside a mounted partition the write is held to the declared region, so
    /// an object holding more data must be refused before touching the disk.
    #[tokio::test]
    #[ignore = "needs root to attach a loop device"]
    async fn object_larger_than_declared_beside_a_mounted_partition_is_refused() {
        let (disk, _guard) = mounted_disk();
        let download_dir = tempdir().unwrap();
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };

        for compressed in [false, true] {
            let data = vec![0x5a; 8192];
            let mut obj =
                raw_object(download_dir.path(), "mlo", &data, compressed, MLO_AT, &disk.device);
            if compressed {
                obj.required_uncompressed_size = 4096;
            } else {
                obj.size = 4096;
            }
            obj.check_requirements(&context).await.unwrap();
            let before = read_at(&disk.device, MLO_AT, data.len());

            let res = obj.install(&context).await;
            assert!(
                matches!(res, Err(Error::LargerThanDeclared(_))),
                "compressed: {compressed}, got {res:?}"
            );
            assert_eq!(read_at(&disk.device, MLO_AT, data.len()), before, "{compressed}");
        }
    }

    /// The check must stop at the end of the object it guards. Objects install
    /// largest first, so the neighbour is already carrying the new version by
    /// the time this one is checked, and reading into it skips an object that
    /// did change.
    #[tokio::test]
    async fn install_if_different_stops_at_the_end_of_the_object() {
        let (obj, download_dir, _source_guard, target_guard) =
            fake_shared_device("2020.01", None, "2020.01");
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };

        obj.install(&context).await.unwrap();

        let written = std::fs::read(target_guard.path()).unwrap();
        assert_eq!(
            written[SHARED_OBJECT_AT..SHARED_OBJECT_AT + SHARED_OBJECT_LEN],
            std::iter::repeat_n(ORIGINAL_BYTE, SHARED_OBJECT_LEN).collect::<Vec<_>>(),
            "the neighbour's version answered for this object and skipped it"
        );
    }

    /// The counterpart: a matching version inside the object's own region still
    /// skips the installation.
    #[tokio::test]
    async fn install_if_different_matches_inside_the_object() {
        let (obj, download_dir, _source_guard, target_guard) =
            fake_shared_device("2020.01", Some("2020.01"), "2019.04");
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };

        let before = std::fs::read(target_guard.path()).unwrap();
        obj.install(&context).await.unwrap();
        let after = std::fs::read(target_guard.path()).unwrap();

        assert_eq!(before, after, "object with an unchanged version should not be installed");
    }

    #[tokio::test]
    async fn raw_full_copy_compressed() {
        let size = 2048;
        let chunk_size = 8;
        let count = definitions::Count::All;
        let seek = 0;
        let skip = 0;
        let truncate = false;
        let compressed = true;

        let (obj, download_dir, _source_guard, target_guard, original_data) =
            fake_raw_object(size, chunk_size, skip, seek, count.clone(), truncate, compressed)
                .unwrap();
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };
        obj.check_requirements(&context).await.unwrap();
        obj.install(&context).await.unwrap();

        validate_file(original_data, target_guard.path(), chunk_size, skip, seek, count)
            .await
            .unwrap();
    }

    #[tokio::test]
    async fn raw_full_copy() {
        let size = 2048;
        let chunk_size = 8;
        let count = definitions::Count::All;
        let seek = 0;
        let skip = 0;
        let truncate = false;
        let compressed = false;

        let (obj, download_dir, _source_guard, target_guard, original_data) =
            fake_raw_object(size, chunk_size, skip, seek, count.clone(), truncate, compressed)
                .unwrap();
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };
        obj.check_requirements(&context).await.unwrap();
        obj.install(&context).await.unwrap();

        validate_file(original_data, target_guard.path(), chunk_size, skip, seek, count)
            .await
            .unwrap();
    }

    #[tokio::test]
    async fn raw_partial_copy_with_skip() {
        let size = 2048;
        let chunk_size = 128;
        let count = definitions::Count::All;
        let seek = 0;
        let skip = 8;
        let truncate = false;
        let compressed = false;

        let (obj, download_dir, _source_guard, target_guard, original_data) =
            fake_raw_object(size, chunk_size, skip, seek, count.clone(), truncate, compressed)
                .unwrap();
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };
        obj.check_requirements(&context).await.unwrap();
        obj.install(&context).await.unwrap();

        validate_file(original_data, target_guard.path(), chunk_size, skip, seek, count)
            .await
            .unwrap();
        check_unwritten_blocks(target_guard.path(), 1024, 1024).await.unwrap();
    }

    #[tokio::test]
    async fn raw_partial_copy_with_seek() {
        let size = 2048;
        let chunk_size = 128;
        let count = definitions::Count::All;
        let seek = 8;
        let skip = 0;
        let truncate = false;
        let compressed = false;

        let (obj, download_dir, _source_guard, target_guard, original_data) =
            fake_raw_object(size, chunk_size, skip, seek, count.clone(), truncate, compressed)
                .unwrap();
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };
        obj.check_requirements(&context).await.unwrap();
        obj.install(&context).await.unwrap();

        validate_file(original_data, target_guard.path(), chunk_size, skip, seek, count)
            .await
            .unwrap();
        check_unwritten_blocks(target_guard.path(), 0, 1024).await.unwrap();
    }

    #[tokio::test]
    async fn raw_partial_copy_with_count() {
        let size = 2048;
        let chunk_size = 128;
        let count = definitions::Count::Limited(8);
        let seek = 0;
        let skip = 0;
        let truncate = false;
        let compressed = false;

        let (obj, download_dir, _source_guard, target_guard, original_data) =
            fake_raw_object(size, chunk_size, skip, seek, count.clone(), truncate, compressed)
                .unwrap();
        let context =
            Context { download_dir: download_dir.path().to_owned(), ..Context::default() };
        obj.check_requirements(&context).await.unwrap();
        obj.install(&context).await.unwrap();

        validate_file(original_data, target_guard.path(), chunk_size, skip, seek, count)
            .await
            .unwrap();
        check_unwritten_blocks(target_guard.path(), 1024, 1024).await.unwrap();
    }
}
