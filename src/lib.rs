//! Your favorite all-in-one formatter tool!

#![deny(missing_docs)]
pub mod command;
pub mod config;
pub mod customlog;
pub mod engine;
pub mod eval_cache;
pub mod formatter;

use anyhow::Result;
use filetime::FileTime;
use path_clean::PathClean;
use serde::{Deserialize, Serialize};
use std::fs::Metadata;
use std::path::PathBuf;
use std::{fmt, path::Path};

/// Mtime represents a unix epoch file modification time
#[derive(Debug, PartialEq, Eq, PartialOrd, Ord, Deserialize, Serialize, Copy, Clone)]
pub struct Mtime(i64);

impl fmt::Display for Mtime {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        self.0.fmt(f)
    }
}

/// Small utility that stat() and retrieve the mtime of a file path
pub fn get_path_mtime(path: &Path) -> Result<Mtime> {
    let metadata = std::fs::metadata(path)?;
    Ok(get_meta_mtime(&metadata))
}

/// Small utility that stat() and retrieve the mtime of a file metadata
#[must_use]
pub fn get_meta_mtime(metadata: &Metadata) -> Mtime {
    Mtime(FileTime::from_last_modification_time(metadata).unix_seconds())
}

/// Returns an absolute path. If the path is absolute already, leave it alone. Otherwise join it to the reference path.
/// Then clean all superfluous ../
#[must_use]
pub fn expand_path(path: &Path, reference: &Path) -> PathBuf {
    let new_path = if path.is_absolute() {
        path.to_path_buf()
    } else {
        reference.join(path)
    };
    new_path.clean()
}

/// Only expands the path if the string contains a slash (/) in it. Otherwise consider it as a string.
pub fn expand_if_path(str: String, reference: &Path) -> String {
    if str.contains('/') {
        expand_path(Path::new(&str), reference)
            .to_string_lossy()
            .to_string()
    } else {
        str
    }
}
