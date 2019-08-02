// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use std::{
    io::{BufReader, BufWriter, Read, Seek, Write},
    os::unix::io::AsRawFd,
    time::Duration,
};
pub(crate) use timeout_readwrite::{TimeoutReader, TimeoutWriter};

pub(crate) fn timed_buf_reader<R>(chunk_size: usize, reader: R) -> BufReader<TimeoutReader<R>>
where
    R: Read + Seek + AsRawFd,
{
    BufReader::with_capacity(
        chunk_size,
        TimeoutReader::new(reader, Duration::from_secs(5)),
    )
}

pub(crate) fn timed_buf_writer<W>(chunk_size: usize, writer: W) -> BufWriter<TimeoutWriter<W>>
where
    W: Write + Seek + AsRawFd,
{
    BufWriter::with_capacity(
        chunk_size,
        TimeoutWriter::new(writer, Duration::from_secs(5)),
    )
}
