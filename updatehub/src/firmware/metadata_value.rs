// Copyright (C) 2017, 2018 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use core::ops::Index;
use serde::Serialize;
use std::{
    collections::{
        btree_map::{Entry, Keys},
        BTreeMap,
    },
    io,
    str::FromStr,
};

#[derive(Debug, PartialEq, Default, Clone)]
pub struct MetadataValue(BTreeMap<String, Vec<String>>);

impl Serialize for MetadataValue {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: serde::Serializer,
    {
        use serde::ser::SerializeMap;

        let mut map = serializer.serialize_map(Some(self.0.len()))?;
        for (k, v) in &self.0 {
            if v.len() == 1 {
                map.serialize_entry(k, &v[0])?;
            } else {
                map.serialize_entry(k, v)?;
            }
        }
        map.end()
    }
}

impl FromStr for MetadataValue {
    type Err = io::Error;

    fn from_str(s: &str) -> io::Result<Self> {
        let mut values = Vec::new();
        for line in s.lines() {
            let v: Vec<_> = line.splitn(2, '=').map(|v| v.trim().to_string()).collect();
            if v.len() != 2 {
                return Err(io::Error::new(
                    io::ErrorKind::InvalidInput,
                    format!(
                        "Invalid format for value '{:?}'. \
                         An <key>=<value> output is \
                         expected",
                        v
                    ),
                ));
            }

            values.push((v[0].clone(), v[1].clone()));
        }
        values.sort();

        let mut mv = Self::default();
        for (k, v) in values {
            mv.entry(k).and_modify(|e| e.push(v.clone())).or_insert_with(|| vec![v]);
        }

        Ok(mv)
    }
}

impl MetadataValue {
    pub fn entry(&mut self, key: String) -> Entry<'_, String, Vec<String>> {
        self.0.entry(key)
    }

    pub fn keys(&self) -> Keys<'_, String, Vec<String>> {
        self.0.keys()
    }

    pub fn is_empty(&self) -> bool {
        self.0.len() == 0
    }

    pub fn len(&self) -> usize {
        self.0.len()
    }
}

impl<'a> Index<&'a str> for MetadataValue {
    type Output = Vec<String>;

    #[inline]
    fn index(&self, key: &str) -> &Vec<String> {
        self.0.get(key).expect("no entry found for key")
    }
}

#[test]
fn serialize() {
    use pretty_assertions::assert_eq;
    let v = MetadataValue::from_str("key1=v1\nkey=b\nnv=\nkey=a").unwrap();

    assert_eq!(serde_json::to_string(&v).unwrap(), r#"{"key":["a","b"],"key1":"v1","nv":""}"#);
}

#[test]
fn valid() {
    use pretty_assertions::assert_eq;
    let v = MetadataValue::from_str("key1=v1\nkey=b\nnv=\nkey=a").unwrap();

    assert_eq!(v.keys().len(), 3);
    assert_eq!(v.keys().collect::<Vec<_>>(), ["key", "key1", "nv"]);
    assert_eq!(v["key1"], ["v1"]);
    assert_eq!(v["key"], ["a", "b"]);
    assert_eq!(v["nv"], [""]);
}

#[test]
fn invalid() {
    assert!(MetadataValue::from_str("\n").is_err());
    assert!(MetadataValue::from_str("key").is_err());
}
