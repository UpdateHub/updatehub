// Copyright (C) 2020 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

use serde::{
    Deserialize, Serialize,
    de::{Deserializer, MapAccess, Visitor},
    ser::{SerializeMap, Serializer},
};
use std::{
    collections::{
        BTreeMap,
        btree_map::{Entry, Keys},
    },
    fmt,
    ops::Index,
    path::PathBuf,
};

/// Metadata stores the firmware metadata information. It is
/// organized in multiple fields.
///
/// The Metadata is created loading its information from the running
/// firmware. It uses the `load` method for that.
#[derive(Clone, Debug, Deserialize, PartialEq, Eq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct Metadata {
    /// Product UID which identifies the firmware on the management system
    pub product_uid: String,
    /// Version of firmware
    pub version: String,
    /// Hardware where the firmware is running
    pub hardware: String,
    /// Path for the pub key being used
    pub pub_key: Option<PathBuf>,
    /// Device Identity
    pub device_identity: MetadataValue,
    /// Device Attributes
    pub device_attributes: MetadataValue,
}

#[derive(Clone, Debug, Default, PartialEq, Eq)]
pub struct MetadataValue(pub BTreeMap<String, Vec<String>>);

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

impl Serialize for MetadataValue {
    fn serialize<S>(&self, serializer: S) -> Result<S::Ok, S::Error>
    where
        S: Serializer,
    {
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

impl<'de> Deserialize<'de> for MetadataValue {
    fn deserialize<D>(deserializer: D) -> Result<Self, D::Error>
    where
        D: Deserializer<'de>,
    {
        #[derive(Deserialize)]
        #[serde(untagged)]
        enum Value {
            One(String),
            Many(Vec<String>),
        }

        impl From<Value> for Vec<String> {
            fn from(value: Value) -> Self {
                match value {
                    Value::One(s) => vec![s],
                    Value::Many(v) => v,
                }
            }
        }

        struct MetadataValueVisitor;

        impl<'de> Visitor<'de> for MetadataValueVisitor {
            type Value = MetadataValue;

            fn expecting(&self, formatter: &mut fmt::Formatter) -> fmt::Result {
                formatter.write_str("tuple struct MetadataValue")
            }

            fn visit_map<M>(self, mut access: M) -> Result<Self::Value, M::Error>
            where
                M: MapAccess<'de>,
            {
                let mut map = MetadataValue::default();

                while let Some((k, v)) = access.next_entry::<_, Value>()? {
                    map.0.insert(k, v.into());
                }

                Ok(map)
            }
        }

        deserializer.deserialize_map(MetadataValueVisitor)
    }
}

impl Index<&str> for MetadataValue {
    type Output = Vec<String>;

    #[inline]
    fn index(&self, key: &str) -> &Vec<String> {
        self.0.get(key).expect("no entry found for key")
    }
}
