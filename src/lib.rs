//! Your favorite all-in-one formatter tool!

#![deny(missing_docs)]
pub mod command;
pub mod config;
pub mod customlog;
pub mod engine;
pub mod eval_cache;

use anyhow::Result;
use customlog::CustomLogOutput;
use filetime::FileTime;
use serde::{Deserialize, Serialize};
use std::cmp::Ordering;
use std::collections::BTreeSet;
use std::fmt;
use std::fs::metadata;
use std::path::PathBuf;
use which::which;

/// The global custom log and user-facing message output.
pub static CLOG: CustomLogOutput = CustomLogOutput::new();

#[derive(Debug, Deserialize, Serialize, Clone)]
/// Each context of the formatter config
pub struct CmdContext {
    /// formatter command to run
    pub command: String,
    /// Last modification time listed in the file's metadata
    pub mtime: i64,
    /// Path to the formatted file
    pub path: PathBuf,
    /// formatter work_dir
    pub work_dir: Option<String>,
    /// formatter arguments or flags
    pub options: Vec<String>,
    /// formatter target path
    pub metadata: BTreeSet<FileMeta>,
}

impl CmdContext {
    /// Update CmdContext mtimes
    pub fn update_meta(self) -> Result<Self> {
        let new_meta: BTreeSet<FileMeta> = self
            .metadata
            .into_iter()
            .map(|e| e.update_mtimes())
            .collect::<Result<BTreeSet<FileMeta>>>()?;
        Ok(CmdContext {
            command: self.command,
            mtime: self.mtime,
            path: self.path,
            work_dir: self.work_dir,
            options: self.options,
            metadata: new_meta,
        })
    }
}

impl PartialEq for CmdContext {
    fn eq(&self, other: &Self) -> bool {
        self.command == other.command
            && self.work_dir == other.work_dir
            && self.options == other.options
            && self.metadata == other.metadata
    }
}

impl Eq for CmdContext {}

#[derive(Debug, Deserialize, Serialize, Clone)]
/// Command metadata created after the first treefmt run
pub struct CmdMeta {
    /// Name provided by user's config
    pub name: String,
    /// Last modification time listed in the file's metadata
    pub mtime: i64,
    /// Path to the formatted file
    pub path: PathBuf,
}

impl CmdMeta {
    /// Create new CmdMeta based on the given config name
    pub fn new(cmd: String) -> Result<Self> {
        let cmd_path = Self::check_bin(&cmd)?;
        let metadata = metadata(&cmd_path)?;
        let cmd_mtime = FileTime::from_last_modification_time(&metadata).unix_seconds();

        Ok(CmdMeta {
            name: cmd,
            mtime: cmd_mtime,
            path: cmd_path,
        })
    }

    /// Make sure that formatter binary exists. This also for other formatter
    fn check_bin<'a>(command: &'a str) -> Result<PathBuf> {
        let cmd_bin = command.split_ascii_whitespace().next().unwrap_or("");
        if let Ok(path) = which(cmd_bin) {
            CLOG.info(&format!("Found {} at {}", cmd_bin, path.display()));
            return Ok(path);
        }
        anyhow::bail!(
            "Failed to locate formatter named {}. \
            Please make sure it is available in your PATH",
            command
        )
    }
}

impl fmt::Display for CmdMeta {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "{}: ({}, {})",
            self.name,
            self.path.display(),
            self.mtime
        )
    }
}

impl PartialEq for CmdMeta {
    fn eq(&self, other: &Self) -> bool {
        self.name == other.name && self.mtime == other.mtime && self.path == other.path
    }
}

impl Eq for CmdMeta {}

#[derive(Debug, Deserialize, Serialize, Clone)]
/// File metadata created after the first treefmt run
pub struct FileMeta {
    /// Last modification time listed in the file's metadata
    pub mtimes: i64,
    /// Path to the formatted file
    pub path: PathBuf,
}

impl FileMeta {
    fn update_mtimes(self) -> Result<Self> {
        let metadata = metadata(&self.path)?;
        let mtime = FileTime::from_last_modification_time(&metadata).unix_seconds();
        Ok(FileMeta {
            mtimes: mtime,
            path: self.path,
        })
    }
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
