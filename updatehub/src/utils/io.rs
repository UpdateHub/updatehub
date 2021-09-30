// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use slog_scope::trace;
use std::pin::Pin;
use std::time::Duration;
use tokio::io::{AsyncRead, AsyncWrite, BufReader, BufWriter};
use tokio_io_timeout::{TimeoutReader, TimeoutWriter};

pub(crate) fn timed_buf_reader<R>(
    chunk_size: usize,
    reader: R,
) -> Pin<Box<BufReader<TimeoutReader<R>>>>
where
    R: AsyncRead,
{
    trace!("starting IO read with 5 seconds of timeout");
    let mut r = TimeoutReader::new(reader);
    r.set_timeout(Some(Duration::from_secs(5)));
    Box::pin(BufReader::with_capacity(chunk_size, r))
}

pub(crate) fn timed_buf_writer<W>(
    chunk_size: usize,
    writer: W,
) -> Pin<Box<BufWriter<TimeoutWriter<W>>>>
where
    W: AsyncWrite,
{
    trace!("starting IO write with 5 seconds of timeout");
    let mut w = TimeoutWriter::new(writer);
    w.set_timeout(Some(Duration::from_secs(5)));
    Box::pin(BufWriter::with_capacity(chunk_size, w))
}
