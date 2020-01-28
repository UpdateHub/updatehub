// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Error, Result};
use crate::{
    object::{Info, Installer},
    utils::{self, definitions::TargetTypeExt},
};

use pkg_schema::{definitions, objects};
use slog_scope::info;
use std::{
    fs,
    io::{BufRead, Seek, SeekFrom, Write},
    path::Path,
};

impl Installer for objects::Raw {
    fn check_requirements(&self) -> Result<()> {
        info!("'raw' handle checking requirements");
        if self.compressed {
            unimplemented!("FIXME: check the required_uncompressed_size");
        }

        if let definitions::TargetType::Device(_) = self.target_type.valid()? {
            return Ok(());
        }

        Err(Error::InvalidTargetType(self.target_type.clone()))
    }

    fn install(&self, download_dir: &Path) -> Result<()> {
        info!("'raw' handler Install");

        let device = match self.target_type {
            definitions::TargetType::Device(ref p) => p,
            _ => unreachable!("Device should be secured by check_requirements"),
        };
        let source = download_dir.join(self.sha256sum());
        let chunk_size = self.chunk_size.0;
        let seek = self.seek * chunk_size as u64;
        let skip = self.skip.0 * chunk_size as u64;
        let truncate = self.truncate.0;
        let count = self.count.clone();

        if self.compressed {
            unimplemented!("FIXME: handle compressed installation");
        } else {
            let mut input = utils::io::timed_buf_reader(chunk_size, fs::File::open(source)?);

            input.seek(SeekFrom::Start(skip))?;
            let mut output = utils::io::timed_buf_writer(
                chunk_size,
                fs::OpenOptions::new().read(true).write(true).truncate(truncate).open(device)?,
            );
            output.seek(SeekFrom::Start(seek))?;

            for _ in count {
                let buf = input.fill_buf()?;
                let len = buf.len();

                // We break the loop in case we have no bytes left for
                // read (EOF is reached).
                if len == 0 {
                    break;
                }

                output.write_all(&buf)?;
                input.consume(len);
            }
        }

        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use pretty_assertions::assert_eq;
    use std::{io, iter, path::PathBuf};
    use tempfile::{tempdir, NamedTempFile};

    const DEFAULT_BYTE: u8 = 0xF;
    const ORIGINAL_BYTE: u8 = 0xA;

    fn fake_raw_object(
        size: u64,
        chunk_size: usize,
        skip: u64,
        seek: u64,
        count: definitions::Count,
        truncate: bool,
    ) -> std::io::Result<(objects::Raw, PathBuf, NamedTempFile, NamedTempFile)> {
        let download_dir = tempdir()?;

        let mut source = NamedTempFile::new_in(download_dir.path())?;
        source.write_all(&iter::repeat(ORIGINAL_BYTE).take(size as usize).collect::<Vec<_>>())?;
        source.seek(SeekFrom::Start(0))?;

        let mut dest = NamedTempFile::new_in(download_dir.path())?;
        dest.write_all(&iter::repeat(DEFAULT_BYTE).take(size as usize).collect::<Vec<_>>())?;
        dest.seek(SeekFrom::Start(0))?;

        Ok((
            objects::Raw {
                filename: "".to_string(),
                size,
                sha256sum: source.path().to_string_lossy().to_string(),
                target_type: definitions::TargetType::Device(dest.path().into()),

                install_if_different: None,
                compressed: false,
                required_uncompressed_size: 0,
                chunk_size: definitions::ChunkSize(chunk_size),
                skip: definitions::Skip(skip),
                seek,
                count,
                truncate: definitions::Truncate(truncate),
            },
            download_dir.into_path(),
            source,
            dest,
        ))
    }

    fn check_unwritten_blocks(f: &mut fs::File, offset: u64, byte_count: u64) -> io::Result<()> {
        let mut f = io::BufReader::with_capacity(1, f);
        f.seek(SeekFrom::Start(offset))?;
        for i in 0..byte_count {
            let buf = f.fill_buf()?;
            let len = buf.len();

            assert_eq!(buf, [DEFAULT_BYTE], "Error on byte {}", i);

            f.consume(len);
        }
        Ok(())
    }

    fn compare_files(
        f1: &mut fs::File,
        f2: &mut fs::File,
        chunk_size: usize,
        skip: u64,
        seek: u64,
        count: definitions::Count,
    ) -> io::Result<()> {
        let mut f1 = io::BufReader::with_capacity(chunk_size, f1);
        f1.seek(SeekFrom::Start(skip * chunk_size as u64))?;
        let mut f2 = io::BufReader::with_capacity(chunk_size, f2);
        f2.seek(SeekFrom::Start(seek * chunk_size as u64))?;

        for _ in count {
            let buf1 = f1.fill_buf()?;
            let len1 = buf1.len();
            let buf2 = f2.fill_buf()?;
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

    #[test]
    fn raw_full_copy() {
        let size = 2048;
        let chunk_size = 8;
        let count = definitions::Count::All;
        let seek = 0;
        let skip = 0;
        let truncate = false;

        let (mut obj, download_dir, mut source_guard, mut target_guard) =
            fake_raw_object(size, chunk_size, skip, seek, count.clone(), truncate).unwrap();
        obj.check_requirements().unwrap();
        obj.setup().unwrap();
        obj.install(&download_dir).unwrap();

        compare_files(
            source_guard.as_file_mut(),
            target_guard.as_file_mut(),
            chunk_size,
            skip,
            seek,
            count.clone(),
        )
        .unwrap();
    }

    #[test]
    fn raw_partial_copy_with_skip() {
        let size = 2048;
        let chunk_size = 128;
        let count = definitions::Count::All;
        let seek = 0;
        let skip = 8;
        let truncate = false;

        let (mut obj, download_dir, mut source_guard, mut target_guard) =
            fake_raw_object(size, chunk_size, skip, seek, count.clone(), truncate).unwrap();
        obj.check_requirements().unwrap();
        obj.setup().unwrap();
        obj.install(&download_dir).unwrap();

        compare_files(
            source_guard.as_file_mut(),
            target_guard.as_file_mut(),
            chunk_size,
            skip,
            seek,
            count.clone(),
        )
        .unwrap();
        check_unwritten_blocks(target_guard.as_file_mut(), 1024, 1024).unwrap();
    }

    #[test]
    fn raw_partial_copy_with_seek() {
        let size = 2048;
        let chunk_size = 128;
        let count = definitions::Count::All;
        let seek = 8;
        let skip = 0;
        let truncate = false;

        let (mut obj, download_dir, mut source_guard, mut target_guard) =
            fake_raw_object(size, chunk_size, skip, seek, count.clone(), truncate).unwrap();
        obj.check_requirements().unwrap();
        obj.setup().unwrap();
        obj.install(&download_dir).unwrap();

        compare_files(
            source_guard.as_file_mut(),
            target_guard.as_file_mut(),
            chunk_size,
            skip,
            seek,
            count.clone(),
        )
        .unwrap();
        check_unwritten_blocks(target_guard.as_file_mut(), 0, 1024).unwrap();
    }

    #[test]
    fn raw_partial_copy_with_count() {
        let size = 2048;
        let chunk_size = 128;
        let count = definitions::Count::Limited(8);
        let seek = 0;
        let skip = 0;
        let truncate = false;

        let (mut obj, download_dir, mut source_guard, mut target_guard) =
            fake_raw_object(size, chunk_size, skip, seek, count.clone(), truncate).unwrap();
        obj.check_requirements().unwrap();
        obj.setup().unwrap();
        obj.install(&download_dir).unwrap();

        compare_files(
            source_guard.as_file_mut(),
            target_guard.as_file_mut(),
            chunk_size,
            skip,
            seek,
            count.clone(),
        )
        .unwrap();
        check_unwritten_blocks(target_guard.as_file_mut(), 1024, 1024).unwrap();
    }
}
