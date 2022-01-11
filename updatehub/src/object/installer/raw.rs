// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0
// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::{Context, Error, Result};
use crate::{
    object::{Info, Installer},
    utils::{self, definitions::TargetTypeExt, log::LogContent},
};
use pkg_schema::{definitions, objects};
use slog_scope::info;
use std::io::SeekFrom;
use tokio::{
    fs,
    io::{AsyncRead, AsyncReadExt, AsyncSeek, AsyncSeekExt},
};
use tokio_take_seek::AsyncTakeSeekExt;

#[async_trait::async_trait(?Send)]
impl Installer for objects::Raw {
    async fn check_requirements(&self, _: &Context) -> Result<()> {
        info!("'raw' handle checking requirements");

        if let definitions::TargetType::Device(dev) =
            self.target_type.valid().log_error_msg("device failed vaidation")?
        {
            utils::fs::ensure_disk_space(dev, self.required_install_size())
                .log_error_msg("not enough disk space")?;
            return Ok(());
        }

        Err(Error::InvalidTargetType(self.target_type.clone()))
    }

    async fn install(&self, context: &Context) -> Result<()> {
        info!("'raw' handler Install {} ({})", self.filename, self.sha256sum);

        let device = match self.target_type {
            definitions::TargetType::Device(ref p) => p,
            _ => unreachable!("device should be secured by check_requirements"),
        };
        let source = context.download_dir.join(self.sha256sum());
        let chunk_size = self.chunk_size.0;
        let seek = self.seek * chunk_size as u64;
        let skip = self.skip.0 * chunk_size as u64;
        let truncate = self.truncate.0;
        let count = self.count.clone();

        let should_skip_install =
            super::should_skip_install(&self.install_if_different, &self.sha256sum, async {
                trait AsyncReadSeek: AsyncRead + AsyncSeek + Unpin {}
                impl<R: AsyncRead + AsyncSeek + Unpin> AsyncReadSeek for R {}

                let h = fs::OpenOptions::new().read(true).open(device).await?;
                let mut h = utils::io::timed_buf_reader(chunk_size, h);
                h.seek(SeekFrom::Start(seek)).await?;
                let h: Box<dyn AsyncReadSeek> = match &count {
                    definitions::Count::All => Box::new(h),
                    definitions::Count::Limited(n) => {
                        Box::new(h.take_with_seek((*n as usize * chunk_size) as u64))
                    }
                };
                Ok(h)
            })
            .await?;
        if should_skip_install {
            return Ok(());
        }

        let mut input: Box<dyn AsyncRead + Unpin> = {
            let mut input = utils::io::timed_buf_reader(
                chunk_size,
                fs::File::open(source).await.log_error_msg("failed to open source file")?,
            );
            input.seek(SeekFrom::Start(skip)).await.log_error_msg("failed to seek source file")?;
            match count {
                definitions::Count::All => Box::new(input),
                definitions::Count::Limited(n) => {
                    Box::new(input.take((n as usize * chunk_size) as u64))
                }
            }
        };
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
            target
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

#[cfg(test)]
mod tests {
    use super::*;
    use flate2::{write::GzEncoder, Compression};
    use pretty_assertions::assert_eq;
    use std::{
        io::{Seek, Write},
        iter,
    };
    use tempfile::{tempdir, NamedTempFile, TempDir};
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
        let original_data = iter::repeat(ORIGINAL_BYTE).take(size as usize).collect::<Vec<_>>();
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
        dest.write_all(&iter::repeat(DEFAULT_BYTE).take(size as usize).collect::<Vec<_>>())?;
        dest.seek(SeekFrom::Start(0))?;

        Ok((
            objects::Raw {
                filename: "".to_string(),
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
        let skip = skip as usize * chunk_size;
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
