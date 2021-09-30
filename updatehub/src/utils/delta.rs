// Copyright (C) 2021 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use super::Result;
use bitar::{Archive, ChunkIndex, CloneOutput, ReaderRemote};
use futures_util::{StreamExt, TryStreamExt};
use slog_scope::trace;
use std::path::Path;
use tokio::{
    fs,
    io::{AsyncSeekExt, AsyncWriteExt},
};

pub(crate) async fn get_required_size(seed: &str, output: &Path) -> Result<u64> {
    let archive_source_size = {
        let seed_as_path = Path::new(seed);

        if seed_as_path.exists() {
            trace!("Loading archive from file: {:?}", seed_as_path);
            let archive = Archive::try_init(fs::File::open(seed_as_path).await?).await?;
            archive.total_source_size()
        } else {
            trace!("Loading archive from url: {}", seed);
            let archive = Archive::try_init(ReaderRemote::from_url(url::Url::parse(seed)?)).await?;
            archive.total_source_size()
        }
    };

    let current_size = fs::metadata(output).await?.len();

    Ok(archive_source_size.checked_sub(current_size).unwrap_or_default())
}

pub(crate) async fn clone(input: &str, output: &Path, output_seek: u64) -> Result<()> {
    let input_as_path = Path::new(input);

    if input_as_path.exists() {
        trace!("Cloning from file: {:?}", input_as_path);
        let archive = Archive::try_init(fs::File::open(input_as_path).await?).await?;
        clone_to_file(archive, output, output_seek).await
    } else {
        trace!("Cloning from url: {}", input);
        let archive = Archive::try_init(ReaderRemote::from_url(url::Url::parse(input)?)).await?;
        clone_to_file(archive, output, output_seek).await
    }
}

async fn clone_to_file<R, E1, E2>(
    mut archive: Archive<R>,
    output: &Path,
    output_seek: u64,
) -> std::result::Result<(), E2>
where
    R: bitar::Reader<Error = E1>,
    E2: From<E1>
        + From<std::io::Error>
        + From<bitar::HashSumMismatchError>
        + From<bitar::CompressionError>,
{
    // Open output file
    let mut output_file =
        fs::OpenOptions::new().create(true).write(true).read(true).open(output).await?;
    output_file.seek(std::io::SeekFrom::Start(output_seek)).await?;

    // Scan the output file for chunks and build a chunk index
    let mut output_index = ChunkIndex::new_empty();
    {
        let chunker = archive.chunker_config().new_chunker(&mut output_file);
        let mut chunk_stream = chunker.map_ok(|(offset, chunk)| (offset, chunk.verify()));
        while let Some(r) = chunk_stream.next().await {
            let (offset, verified) = r?;
            let (hash, chunk) = verified.into_parts();
            output_index.add_chunk(hash, chunk.len(), &[offset]);
        }
    }

    // Create output to contain the clone of the archive's source
    let mut output = CloneOutput::new(&mut output_file, archive.build_source_index());

    // Reorder chunks in the output
    let _reused_bytes = output.reorder_in_place(output_index).await?;

    // Fetch the rest of the chunks from the archive
    let mut chunk_stream = archive.chunk_stream(output.chunks());
    // let mut read_archive_bytes = 0;
    while let Some(result) = chunk_stream.next().await {
        let compressed = result?;
        // read_archive_bytes += compressed.len();
        let unverified = compressed.decompress()?;
        let verified = unverified.verify()?;
        output.feed(&verified).await?;
    }

    // Ensure that the output file has been fully updated before returning
    output_file.flush().await?;

    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_inplace_clone() {
        // Create file with incomplete data
        let output_file = tempfile::NamedTempFile::new().unwrap();
        fs::write(output_file.path(), "Test message sample: [...], in Rio.").await.unwrap();

        clone("fixtures/message.bita", output_file.path(), 0).await.unwrap();

        // Assert that the file now has the full predefined message
        assert_eq!(
            fs::read_to_string(output_file.path()).await.unwrap().as_str(),
            "Test message sample: The Brazilian campaign at the Tokyo Games ended with positive results. With 21 medals, the country surpassed the record of 19 registered in 2016, in Rio.",
        );
    }
}
