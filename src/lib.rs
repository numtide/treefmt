//! Your favorite all-in-one formatter tool!

#![deny(missing_docs)]
pub mod command;
pub mod config;
pub mod customlog;
pub mod engine;
pub mod eval_cache;

use customlog::CustomLogOutput;
use serde::{Deserialize, Serialize};
use std::cmp::Ordering;
use std::collections::BTreeSet;
use std::path::PathBuf;

/// The global custom log and user-facing message output.
pub static CLOG: CustomLogOutput = CustomLogOutput::new();

#[derive(Debug, Deserialize, Serialize, Clone)]
/// Each context of the formatter config
pub struct CmdContext {
    /// formatter command to run
    pub command: String,
    /// formatter workdir
    pub workdir: String,
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
/// File metadata created after the first treefmt run
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
