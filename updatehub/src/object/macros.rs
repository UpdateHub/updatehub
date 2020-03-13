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
        }
    };
}

macro_rules! impl_object_info {
    ($objtype:ty) => {
        impl Info for $objtype {
            fn filename(&self) -> &str {
                &self.filename
            }

            fn len(&self) -> u64 {
                self.size
            }

            fn sha256sum(&self) -> &str {
                &self.sha256sum
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
            Object::Raw($alias) => $code,
            Object::Tarball($alias) => $code,
            Object::Test($alias) => $code,
            Object::Ubifs($alias) => $code,
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
                    slog_scope::info!("Skipping installation due to install if different rule");
                    return Ok(());
                }
                Ok(false) => {
                    slog_scope::debug!("Install if different reported difference");
                }
                Err(e) => {
                    slog_scope::info!("Install if different reported error: {}", e);
                }
            },
            None => slog_scope::debug!("No install if different rule set, proceeding"),
        }
    };
}
