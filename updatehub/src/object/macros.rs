// Copyright (C) 2019 O.S. Systems Sofware LTDA
//
// SPDX-License-Identifier: Apache-2.0

macro_rules! impl_object_for_object_types {
    ( $( $objtype:ident ),* ) => {
        impl Info for Object {
            fn status(&self, download_dir: &std::path::Path) -> crate::object::Result<Status> {
                match *self {
                    $( Object::$objtype(ref o) => Ok(o.status(download_dir)?), )*
                }
            }

            fn mode(&self) -> String {
                match *self {
                    $( Object::$objtype(ref o) => o.mode(), )*
                }
            }

            fn filename(&self) -> &str {
                match *self {
                    $( Object::$objtype(ref o) => o.filename(), )*
                }
            }

            fn len(&self) -> u64 {
                match *self {
                    $( Object::$objtype(ref o) => o.len(), )*
                }
            }

            fn sha256sum(&self) -> &str {
                match *self {
                    $( Object::$objtype(ref o) => o.sha256sum(), )*
                }
            }

            fn required_install_size(&self) -> u64 {
                match *self {
                    $( Object::$objtype(ref o) => o.required_install_size(), )*
                }
            }
        }
    };
}

macro_rules! impl_object_info {
    ($objtype:ty) => {
        impl Info for $objtype {
            fn mode(&self) -> String {
                stringify!($objtype).to_lowercase()
            }

            fn filename(&self) -> &str {
                &self.filename
            }

            fn len(&self) -> u64 {
                self.size
            }

            fn sha256sum(&self) -> &str {
                &self.sha256sum
            }

            fn required_install_size(&self) -> u64 {
                self.size
            }
        }
    };
}

macro_rules! impl_compressed_object_info {
    ($objtype:ty) => {
        impl Info for $objtype {
            fn mode(&self) -> String {
                stringify!($objtype).to_lowercase()
            }

            fn filename(&self) -> &str {
                &self.filename
            }

            fn len(&self) -> u64 {
                self.size
            }

            fn sha256sum(&self) -> &str {
                &self.sha256sum
            }

            fn required_install_size(&self) -> u64 {
                if self.compressed { self.required_uncompressed_size } else { self.size }
            }
        }
    };
}

macro_rules! for_any_object {
    ($mode:ident, $alias:ident, $code:block) => {
        match $mode {
            Object::Copy($alias) => $code,
            Object::Flash($alias) => $code,
            Object::Imxkobs($alias) => $code,
            Object::Mender($alias) => $code,
            Object::Raw($alias) => $code,
            Object::Tarball($alias) => $code,
            Object::Test($alias) => $code,
            Object::Ubifs($alias) => $code,
            Object::UbootEnv($alias) => $code,
            Object::Zephyr($alias) => $code,
        }
    };
}

// This wil execute the block only if the $rule is present, if so, the block
// must return a `R` that implements `Read` and `Seek`. If the
// `check_if_different` reports `Ok(true)` for `R` return is called.
macro_rules! handle_install_if_different {
    ($rule:expr, $sha256sum:expr, $handler:block) => {
        match $rule {
            Some(ref rule) => match $handler.and_then(|mut h| {
                crate::object::installer::check_if_different(&mut h, rule, $sha256sum)
            }) {
                Ok(true) => {
                    slog_scope::info!(
                        "installation of {} has been skipped (install if different): {}",
                        $sha256sum,
                        $rule.as_ref().unwrap()
                    );
                    return Ok(());
                }
                Ok(false) => {
                    slog_scope::debug!(
                        "installation will proceed (installation if different): {}",
                        $rule.as_ref().unwrap()
                    );
                }
                Err(e) => {
                    slog_scope::error!(
                        "install if different check ({}) check failed, error: {}",
                        $rule.as_ref().unwrap(),
                        e
                    );
                }
            },
            None => slog_scope::trace!("no install if different rule set, proceeding"),
        }
    };
}
