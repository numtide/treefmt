//! Your favorite all-in-one formatter tool!

#![deny(missing_docs)]
pub mod command;
pub mod customlog;
pub mod engine;
pub mod formatters;

use customlog::CustomLogOutput;
use serde::{Deserialize, Serialize};
use std::cmp::Ordering;
use std::collections::{BTreeMap, BTreeSet};
use std::path::PathBuf;

/// The global custom log and user-facing message output.
pub static CLOG: CustomLogOutput = CustomLogOutput::new();

/// prjfmt.toml structure
#[derive(Debug, Deserialize)]
pub struct Root {
    /// Map of formatters into the config
    pub formatters: BTreeMap<String, FmtConfig>,
}

/// Config for each formatters
#[derive(Debug, Deserialize)]
pub struct FmtConfig {
    /// File extensions that want to be formatted
    pub files: FileExtensions,
    /// File or Folder that is included to be formatted
    pub includes: Option<Vec<String>>,
    /// File or Folder that is excluded to be formatted
    pub excludes: Option<Vec<String>>,
    /// Command formatter to run
    pub command: Option<String>,
    /// Argument for formatter
    pub options: Option<Vec<String>>,
}

/// File extensions can be single string (e.g. "*.hs") or
/// list of string (e.g. [ "*.hs", "*.rs" ])
#[derive(Debug, Deserialize, Clone)]
#[serde(untagged)]
pub enum FileExtensions {
    /// Single file type
    SingleFile(String),
    /// List of file type
    MultipleFile(Vec<String>),
}

impl<'a> IntoIterator for &'a FileExtensions {
    type Item = &'a String;
    type IntoIter = either::Either<std::iter::Once<&'a String>, std::slice::Iter<'a, String>>;

    fn into_iter(self) -> Self::IntoIter {
        match self {
            FileExtensions::SingleFile(glob) => either::Either::Left(std::iter::once(glob)),
            FileExtensions::MultipleFile(globs) => either::Either::Right(globs.iter()),
        }
    }
}

#[derive(Debug, Deserialize, Serialize, Clone)]
/// Each context of the formatter config
pub struct CmdContext {
    /// formatter command to run
    pub command: String,
    /// formatter arguments or flags
    pub options: Vec<String>,
    /// formatter target path
    pub metadata: BTreeSet<FileMeta>,
}

impl PartialEq for CmdContext {
    fn eq(&self, other: &Self) -> bool {
        self.command == other.command
            && self.options == other.options
            && self.metadata == other.metadata
    }
}

impl Eq for CmdContext {}

#[derive(Debug, Deserialize, Serialize, Clone)]
/// File metadata created after the first prjfmt run
pub struct FileMeta {
    /// Last modification time listed in the file's metadata
    pub mtimes: i64,
    /// Path to the formatted file
    pub path: PathBuf,
}

impl Ord for FileMeta {
    fn cmp(&self, other: &Self) -> Ordering {
        if self.eq(other) {
            return Ordering::Equal;
        }
        if self.mtimes.eq(&other.mtimes) {
            return self.path.cmp(&other.path);
        }
        self.mtimes.cmp(&other.mtimes)
    }
}

impl PartialOrd for FileMeta {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl PartialEq for FileMeta {
    fn eq(&self, other: &Self) -> bool {
        self.mtimes == other.mtimes && self.path == other.path
    }
}

impl Eq for FileMeta {}
