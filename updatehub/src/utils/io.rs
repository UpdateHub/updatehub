// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use slog_scope::trace;
use std::{
    io,
    pin::Pin,
    task::{Context, Poll, ready},
    time::Duration,
};
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

/// Fails a write going past `limit` bytes, so an object cannot reach beyond
/// the region its requirements were checked for.
pub(crate) struct BoundedWriter<W> {
    inner: W,
    remaining: u64,
}

impl<W> BoundedWriter<W> {
    pub(crate) fn new(inner: W, limit: u64) -> Self {
        BoundedWriter { inner, remaining: limit }
    }
}

impl<W: AsyncWrite + Unpin> AsyncWrite for BoundedWriter<W> {
    fn poll_write(
        mut self: Pin<&mut Self>,
        cx: &mut Context<'_>,
        buf: &[u8],
    ) -> Poll<io::Result<usize>> {
        if self.remaining == 0 && !buf.is_empty() {
            return Poll::Ready(Err(io::Error::other(
                "data goes past the region checked for the object",
            )));
        }

        let len = buf.len().min(usize::try_from(self.remaining).unwrap_or(usize::MAX));
        let written = ready!(Pin::new(&mut self.inner).poll_write(cx, &buf[..len]))?;
        self.remaining -= written as u64;

        Poll::Ready(Ok(written))
    }

    fn poll_flush(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<io::Result<()>> {
        Pin::new(&mut self.inner).poll_flush(cx)
    }

    fn poll_shutdown(mut self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<io::Result<()>> {
        Pin::new(&mut self.inner).poll_shutdown(cx)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use tokio::io::AsyncWriteExt;

    #[tokio::test]
    async fn bounded_writer_takes_up_to_its_limit() {
        let mut out = Vec::new();
        let mut w = BoundedWriter::new(&mut out, 4);
        w.write_all(b"abcd").await.unwrap();
        w.flush().await.unwrap();

        assert_eq!(out, b"abcd");
    }

    #[tokio::test]
    async fn bounded_writer_refuses_what_goes_past_its_limit() {
        let mut out = Vec::new();
        let mut w = BoundedWriter::new(&mut out, 4);
        let res = w.write_all(b"abcdef").await;

        assert!(res.is_err(), "writing past the limit must fail");
        assert_eq!(out, b"abcd");
    }
}
