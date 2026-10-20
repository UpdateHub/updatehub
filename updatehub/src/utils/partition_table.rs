// Copyright (C) 2026 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

//! Locates the partition tables of a disk. They lie outside every partition,
//! yet damaging them takes every partition with them.

use slog_scope::warn;
use std::{fs::File, ops::Range, os::unix::fs::FileExt};

/// The disk signature, the partition entries and the boot signature. The boot
/// code ahead of them is left for a BIOS loader to replace.
const MBR_TABLE: Range<u64> = 440..512;
const EXTENDED_TYPES: [u8; 3] = [0x05, 0x0f, 0x85];
const GPT_PROTECTIVE_TYPE: u8 = 0xee;
const GPT_SIGNATURE: &[u8] = b"EFI PART";
/// A corrupted chain of EBRs may loop, so its walk is bounded.
const MAX_LOGICAL_PARTITIONS: usize = 256;
/// Far above the 16 KiB a GPT usually reserves for its entries.
const MAX_GPT_ENTRIES_SIZE: u64 = 1024 * 1024;

/// Byte ranges of `disk` holding its MBR, EBRs and GPT. `sector_size` is the
/// logical block size, `disk_size` is in bytes.
///
/// What cannot be read or makes no sense holds no partition in place, so it
/// is left out, while the parts of a table which do make sense are kept.
pub(crate) fn protected_ranges(disk: &File, sector_size: u64, disk_size: u64) -> Vec<Range<u64>> {
    let mut ranges = Vec::new();
    if sector_size < 512 || !sector_size.is_power_of_two() {
        warn!("unsupported logical block size {}, partition table left unprotected", sector_size);
        return ranges;
    }

    let disk = Disk { file: disk, sector_size, sectors: disk_size / sector_size };
    let Some(mbr) = disk.read(0, 512).filter(|mbr| has_boot_signature(mbr)) else {
        return ranges;
    };
    ranges.push(MBR_TABLE);

    for entry in mbr_entries(&mbr).filter(|entry| EXTENDED_TYPES.contains(&entry.kind)) {
        disk.ebrs(&entry, &mut ranges);
    }
    // The kernel only looks for a GPT behind a protective or hybrid MBR, a
    // stale GPT header on an MBR disk holds nothing in place.
    if mbr_entries(&mbr).any(|entry| entry.kind == GPT_PROTECTIVE_TYPE) {
        disk.gpt(&mut ranges);
    }

    ranges
}

struct Disk<'a> {
    file: &'a File,
    sector_size: u64,
    sectors: u64,
}

impl Disk<'_> {
    /// Reads `len` bytes from sector `lba`, which must lie on the disk.
    fn read(&self, lba: u64, len: usize) -> Option<Vec<u8>> {
        if lba >= self.sectors {
            warn!("partition table at sector {} lies past the end of the disk", lba);
            return None;
        }
        let mut buf = vec![0; len];
        match self.file.read_exact_at(&mut buf, lba * self.sector_size) {
            Ok(()) => Some(buf),
            Err(e) => {
                warn!("failed to read the partition table at sector {}: {}", lba, e);
                None
            }
        }
    }

    fn sectors_range(&self, lba: u64, count: u64) -> Range<u64> {
        lba * self.sector_size..(lba + count) * self.sector_size
    }

    /// Walks the chain of EBRs of an extended partition. Each EBR sits in
    /// the sector ahead of its logical partition and links to the next one,
    /// relative to the start of the extended partition.
    fn ebrs(&self, extended: &MbrEntry, ranges: &mut Vec<Range<u64>>) {
        let container = extended.start..extended.start.saturating_add(extended.sectors);
        let mut next = container.start;

        for _ in 0..MAX_LOGICAL_PARTITIONS {
            if !container.contains(&next) {
                warn!("EBR at sector {} lies outside its extended partition", next);
                return;
            }
            let Some(ebr) = self.read(next, 512).filter(|ebr| has_boot_signature(ebr)) else {
                return;
            };
            ranges.push(self.sectors_range(next, 1));

            match mbr_entries(&ebr).nth(1) {
                Some(link) if EXTENDED_TYPES.contains(&link.kind) => {
                    next = container.start.saturating_add(link.start);
                }
                _ => return,
            }
        }

        warn!("chain of EBRs longer than {} logical partitions", MAX_LOGICAL_PARTITIONS);
    }

    /// The primary GPT and its backup. As the kernel does, the backup is
    /// looked for where the primary says, or on the last sector when the
    /// primary is unusable.
    fn gpt(&self, ranges: &mut Vec<Range<u64>>) {
        let primary = self.gpt_header(1, ranges);
        let alternate =
            primary.as_ref().map_or(self.sectors.saturating_sub(1), |primary| primary.alternate);
        let backup = self.gpt_header(alternate, ranges);

        for header in primary.iter().chain(&backup) {
            let count = header.entries_len.div_ceil(self.sector_size);
            if header.entries_lba.checked_add(count).is_none_or(|end| end > self.sectors) {
                warn!("GPT entries at sector {} lie past the end of the disk", header.entries_lba);
            } else {
                ranges.push(self.sectors_range(header.entries_lba, count));
            }
        }
    }

    /// The header at `lba`. Carrying the GPT signature is enough to protect
    /// its sector, its fields have to make sense to locate its entries.
    fn gpt_header(&self, lba: u64, ranges: &mut Vec<Range<u64>>) -> Option<GptHeader> {
        let raw = self.read(lba, 92).filter(|raw| raw.starts_with(GPT_SIGNATURE))?;
        ranges.push(self.sectors_range(lba, 1));

        let header = GptHeader::parse(&raw);
        if header.is_none() {
            warn!("GPT header at sector {} does not describe its entries", lba);
        }
        header
    }
}

struct MbrEntry {
    kind: u8,
    start: u64,
    sectors: u64,
}

fn mbr_entries(sector: &[u8]) -> impl Iterator<Item = MbrEntry> + '_ {
    sector[446..510].chunks_exact(16).map(|entry| MbrEntry {
        kind: entry[4],
        start: le32(entry, 8),
        sectors: le32(entry, 12),
    })
}

fn has_boot_signature(sector: &[u8]) -> bool {
    sector[510..512] == [0x55, 0xaa]
}

struct GptHeader {
    alternate: u64,
    entries_lba: u64,
    /// The whole array the header declares, its checksum covering the
    /// unused entries too.
    entries_len: u64,
}

impl GptHeader {
    fn parse(raw: &[u8]) -> Option<GptHeader> {
        let entry_size = le32(raw, 84);
        if entry_size < 128 || entry_size % 8 != 0 {
            return None;
        }

        Some(GptHeader {
            alternate: le64(raw, 32),
            entries_lba: le64(raw, 72),
            entries_len: le32(raw, 80)
                .checked_mul(entry_size)
                .filter(|len| *len <= MAX_GPT_ENTRIES_SIZE && *len > 0)?,
        })
    }
}

fn le32(bytes: &[u8], at: usize) -> u64 {
    u32::from_le_bytes(bytes[at..at + 4].try_into().unwrap()).into()
}

fn le64(bytes: &[u8], at: usize) -> u64 {
    u64::from_le_bytes(bytes[at..at + 8].try_into().unwrap())
}

#[cfg(test)]
pub(crate) mod tests {
    use super::*;
    use pretty_assertions::assert_eq;
    use std::io;

    pub(crate) const LINUX: u8 = 0x83;
    pub(crate) const EXTENDED: u8 = 0x05;

    /// Writes an MBR holding `entries`, as `(type, start, sectors)`.
    pub(crate) fn write_mbr(file: &File, entries: &[(u8, u64, u64)]) -> io::Result<()> {
        write_mbr_at(file, 0, entries)
    }

    fn write_mbr_at(file: &File, offset: u64, entries: &[(u8, u64, u64)]) -> io::Result<()> {
        for (i, (kind, start, sectors)) in (0u64..).zip(entries) {
            let mut entry = [0u8; 16];
            entry[4] = *kind;
            entry[8..12].copy_from_slice(&u32::try_from(*start).unwrap().to_le_bytes());
            entry[12..16].copy_from_slice(&u32::try_from(*sectors).unwrap().to_le_bytes());
            file.write_all_at(&entry, offset + 446 + 16 * i)?;
        }
        file.write_all_at(&[0x55, 0xaa], offset + 510)
    }

    /// Writes the EBR at `ebr`, an offset into the extended partition at
    /// `extended`, describing a logical partition of `sectors` from
    /// `start` sectors after the EBR and linking to the EBR at `next`.
    pub(crate) fn write_ebr(
        file: &File,
        sector_size: u64,
        extended: u64,
        ebr: u64,
        (start, sectors): (u64, u64),
        next: Option<u64>,
    ) -> io::Result<()> {
        let mut entries = vec![(LINUX, start, sectors)];
        entries.extend(next.map(|next| (EXTENDED, next, start + sectors)));
        write_mbr_at(file, (extended + ebr) * sector_size, &entries)
    }

    /// Lays out a GPT the kernel accepts: a protective MBR, the primary
    /// header with its entries at `entries_lba` and the backup at the end.
    pub(crate) struct Gpt {
        pub(crate) sector_size: u64,
        pub(crate) sectors: u64,
        pub(crate) entries_lba: u64,
        pub(crate) entries: u32,
        /// First and last sector of each partition.
        pub(crate) partitions: Vec<(u64, u64)>,
    }

    impl Gpt {
        pub(crate) fn new(sector_size: u64, sectors: u64, partitions: Vec<(u64, u64)>) -> Gpt {
            Gpt { sector_size, sectors, entries_lba: 2, entries: 128, partitions }
        }

        fn array_sectors(&self) -> u64 {
            (u64::from(self.entries) * 128).div_ceil(self.sector_size)
        }

        pub(crate) fn write(&self, file: &File) -> io::Result<()> {
            let last = self.sectors - 1;
            write_mbr(file, &[(GPT_PROTECTIVE_TYPE, 1, last.min(u64::from(u32::MAX)))])?;

            let mut array = vec![0u8; usize::try_from(self.entries).unwrap() * 128];
            for (entry, (first, last)) in array.chunks_exact_mut(128).zip(&self.partitions) {
                // Linux filesystem data, any non-zero type would do.
                entry[0..16].copy_from_slice(&[
                    0xaf, 0x3d, 0xc6, 0x0f, 0x83, 0x84, 0x72, 0x47, 0x8e, 0x79, 0x3d, 0x69, 0xd8,
                    0x47, 0x7d, 0xe4,
                ]);
                entry[16] = 1;
                entry[32..40].copy_from_slice(&first.to_le_bytes());
                entry[40..48].copy_from_slice(&last.to_le_bytes());
            }
            let backup_entries = last - self.array_sectors();
            let first_usable = (self.entries_lba + self.array_sectors()).max(34);

            for (lba, alternate, entries_lba) in
                [(1, last, self.entries_lba), (last, 1, backup_entries)]
            {
                let mut header = vec![0u8; 92];
                header[0..8].copy_from_slice(GPT_SIGNATURE);
                header[8..12].copy_from_slice(&0x0001_0000u32.to_le_bytes());
                header[12..16].copy_from_slice(&92u32.to_le_bytes());
                header[24..32].copy_from_slice(&lba.to_le_bytes());
                header[32..40].copy_from_slice(&alternate.to_le_bytes());
                header[40..48].copy_from_slice(&first_usable.to_le_bytes());
                header[48..56].copy_from_slice(&(backup_entries - 1).to_le_bytes());
                header[56] = 1;
                header[72..80].copy_from_slice(&entries_lba.to_le_bytes());
                header[80..84].copy_from_slice(&self.entries.to_le_bytes());
                header[84..88].copy_from_slice(&128u32.to_le_bytes());
                header[88..92].copy_from_slice(&crc32(&array).to_le_bytes());
                let crc = crc32(&header);
                header[16..20].copy_from_slice(&crc.to_le_bytes());

                file.write_all_at(&header, lba * self.sector_size)?;
                file.write_all_at(&array, entries_lba * self.sector_size)?;
            }

            Ok(())
        }
    }

    fn crc32(data: &[u8]) -> u32 {
        let mut crc = !0u32;
        for byte in data {
            crc ^= u32::from(*byte);
            for _ in 0..8 {
                crc = if crc & 1 == 0 { crc >> 1 } else { (crc >> 1) ^ 0xedb8_8320 };
            }
        }
        !crc
    }

    const SIZE: u64 = 64 * 1024 * 1024;

    fn image() -> tempfile::NamedTempFile {
        let file = tempfile::NamedTempFile::new().unwrap();
        file.as_file().set_len(SIZE).unwrap();
        file
    }

    fn ranges_of(file: &tempfile::NamedTempFile, sector_size: u64) -> Vec<Range<u64>> {
        let mut ranges = protected_ranges(file.as_file(), sector_size, SIZE);
        ranges.sort_by_key(|range| (range.start, range.end));
        ranges.dedup();
        ranges
    }

    #[test]
    fn crc32_matches_the_reference_value() {
        assert_eq!(crc32(b"123456789"), 0xcbf4_3926);
    }

    #[test]
    fn blank_disk_has_nothing_to_protect() {
        assert_eq!(ranges_of(&image(), 512), vec![]);
    }

    #[test]
    fn mbr_protects_its_table_but_not_the_boot_code() {
        let file = image();
        write_mbr(file.as_file(), &[(LINUX, 2048, 8192)]).unwrap();

        assert_eq!(ranges_of(&file, 512), vec![440..512]);
    }

    #[test]
    fn stale_gpt_on_an_mbr_disk_is_ignored() {
        let file = image();
        Gpt::new(512, SIZE / 512, vec![(2048, 10239)]).write(file.as_file()).unwrap();
        write_mbr(file.as_file(), &[(LINUX, 2048, 8192)]).unwrap();

        assert_eq!(ranges_of(&file, 512), vec![440..512]);
    }

    #[test]
    fn every_ebr_of_an_extended_partition_is_protected() {
        let file = image();
        let f = file.as_file();
        write_mbr(f, &[(LINUX, 2048, 8192), (EXTENDED, 16384, 65536)]).unwrap();
        write_ebr(f, 512, 16384, 0, (2048, 8192), Some(16384)).unwrap();
        write_ebr(f, 512, 16384, 16384, (2048, 8192), Some(32768)).unwrap();
        write_ebr(f, 512, 16384, 32768, (2048, 8192), None).unwrap();

        assert_eq!(
            ranges_of(&file, 512),
            vec![
                440..512,
                16384 * 512..16385 * 512,
                32768 * 512..32769 * 512,
                49152 * 512..49153 * 512
            ]
        );
    }

    #[test]
    fn looping_chain_of_ebrs_keeps_what_it_walked() {
        let file = image();
        let f = file.as_file();
        write_mbr(f, &[(EXTENDED, 16384, 65536)]).unwrap();
        write_ebr(f, 512, 16384, 0, (2048, 8192), Some(16384)).unwrap();
        write_ebr(f, 512, 16384, 16384, (2048, 8192), Some(0)).unwrap();

        assert_eq!(
            ranges_of(&file, 512),
            vec![440..512, 16384 * 512..16385 * 512, 32768 * 512..32769 * 512]
        );
    }

    #[test]
    fn ebr_linking_outside_its_extended_partition_ends_the_chain() {
        let file = image();
        let f = file.as_file();
        write_mbr(f, &[(EXTENDED, 16384, 32768)]).unwrap();
        write_ebr(f, 512, 16384, 0, (2048, 8192), Some(65536)).unwrap();

        assert_eq!(ranges_of(&file, 512), vec![440..512, 16384 * 512..16385 * 512]);
    }

    #[test]
    fn gpt_protects_both_headers_and_their_entries() {
        let sectors = SIZE / 512;
        let file = image();
        Gpt::new(512, sectors, vec![(2048, sectors - 34)]).write(file.as_file()).unwrap();

        assert_eq!(
            ranges_of(&file, 512),
            vec![
                440..512,
                512..1024,
                1024..34 * 512,
                (sectors - 33) * 512..(sectors - 1) * 512,
                (sectors - 1) * 512..SIZE,
            ]
        );
    }

    #[test]
    fn gpt_follows_the_logical_block_size() {
        let sectors = SIZE / 4096;
        let file = image();
        Gpt::new(4096, sectors, vec![(256, sectors - 6)]).write(file.as_file()).unwrap();

        assert_eq!(
            ranges_of(&file, 4096),
            vec![
                440..512,
                4096..8192,
                8192..6 * 4096,
                (sectors - 5) * 4096..(sectors - 1) * 4096,
                (sectors - 1) * 4096..SIZE,
            ]
        );
    }

    #[test]
    fn gpt_protects_only_the_entries_it_declares() {
        let sectors = SIZE / 512;
        let file = image();
        let gpt = Gpt { entries: 4, ..Gpt::new(512, sectors, vec![(2048, sectors - 3)]) };
        gpt.write(file.as_file()).unwrap();

        assert_eq!(
            ranges_of(&file, 512),
            vec![
                440..512,
                512..1024,
                1024..1536,
                (sectors - 2) * 512..(sectors - 1) * 512,
                (sectors - 1) * 512..SIZE,
            ]
        );
    }

    #[test]
    fn gpt_entries_may_be_relocated() {
        let sectors = SIZE / 512;
        let file = image();
        let gpt = Gpt { entries_lba: 2016, ..Gpt::new(512, sectors, vec![(2048, sectors - 34)]) };
        gpt.write(file.as_file()).unwrap();

        assert_eq!(ranges_of(&file, 512)[2], 2016 * 512..2048 * 512);
    }

    #[test]
    fn missing_backup_gpt_leaves_the_primary_protected() {
        let sectors = SIZE / 512;
        let file = image();
        Gpt::new(512, sectors, vec![(2048, sectors - 34)]).write(file.as_file()).unwrap();
        file.as_file().write_all_at(&[0; 512], SIZE - 512).unwrap();

        assert_eq!(ranges_of(&file, 512), vec![440..512, 512..1024, 1024..34 * 512]);
    }

    #[test]
    fn gpt_header_making_no_sense_protects_only_its_sector() {
        let sectors = SIZE / 512;
        let file = image();
        Gpt::new(512, sectors, vec![(2048, sectors - 34)]).write(file.as_file()).unwrap();
        // An entry size of zero, and no backup to fall back to.
        file.as_file().write_all_at(&[0; 4], 512 + 84).unwrap();
        file.as_file().write_all_at(&[0; 512], SIZE - 512).unwrap();

        assert_eq!(ranges_of(&file, 512), vec![440..512, 512..1024]);
    }

    #[test]
    fn unsupported_block_size_protects_nothing() {
        let file = image();
        write_mbr(file.as_file(), &[(LINUX, 2048, 8192)]).unwrap();

        assert_eq!(ranges_of(&file, 0), vec![]);
        assert_eq!(ranges_of(&file, 1000), vec![]);
    }
}
