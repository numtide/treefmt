//! Your favorite all-in-one formatter tool!

#![deny(missing_docs)]
pub mod command;
pub mod config;
pub mod customlog;
pub mod engine;
pub mod eval_cache;

use crate::eval_cache::{get_mtime, Mtime};
use anyhow::Result;
use customlog::CustomLogOutput;
use serde::{Deserialize, Serialize};
use std::cmp::Ordering;
use std::collections::BTreeSet;
use std::fmt;
use std::path::PathBuf;

/// The global custom log and user-facing message output.
pub static CLOG: CustomLogOutput = CustomLogOutput::new();

#[derive(Debug, Deserialize, Serialize, Clone)]
/// Each context of the formatter config
pub struct CmdContext {
    /// Name of the command
    pub name: String,
    /// Path to the formatted file
    pub path: PathBuf,
    /// Last modification time listed in the file's metadata
    pub mtime: Mtime,
    /// formatter work_dir
    pub work_dir: PathBuf,
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
            .map(|e| e.update_mtime())
            .collect::<Result<BTreeSet<FileMeta>>>()?;
        Ok(CmdContext {
            name: self.name,
            path: self.path,
            mtime: self.mtime,
            work_dir: self.work_dir,
            options: self.options,
            metadata: new_meta,
        })
    }
}

impl PartialEq for CmdContext {
    fn eq(&self, other: &Self) -> bool {
        self.name == other.name
            && self.path == other.path
            && self.work_dir == other.work_dir
            && self.options == other.options
            && self.metadata == other.metadata
    }
}

impl Eq for CmdContext {}

#[derive(Debug, Deserialize, Serialize, Clone)]
/// Command metadata created after the first treefmt run
pub struct CmdMeta {
    /// Absolute path to the formatter
    pub path: PathBuf,
    /// Last modification time listed in the file's metadata
    pub mtime: Mtime,
}

impl CmdMeta {
    /// Create new CmdMeta based on the given config name
    ///
    /// We assume that cmd_path is absolute.
    pub fn new(cmd_path: &PathBuf) -> Result<Self> {
        assert!(cmd_path.is_absolute());
        Ok(CmdMeta {
            path: cmd_path.clone(),
            mtime: get_mtime(&cmd_path)?,
        })
    }
}

impl fmt::Display for CmdMeta {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "({}, {})", self.path.display(), self.mtime)
    }
}

impl PartialEq for CmdMeta {
    fn eq(&self, other: &Self) -> bool {
        self.path == other.path && self.mtime == other.mtime
    }
}

impl Eq for CmdMeta {}

#[derive(Debug, Deserialize, Serialize, Clone)]
/// File metadata created after the first treefmt run
pub struct FileMeta {
    /// Path to the formatted file
    pub path: PathBuf,
    /// Last modification time listed in the file's metadata
    pub mtime: Mtime,
}

impl FileMeta {
    fn update_mtime(self) -> Result<Self> {
        let mtime = get_mtime(&self.path)?;
        Ok(FileMeta {
            path: self.path,
            mtime,
        })
    }
}

impl Ord for FileMeta {
    fn cmp(&self, other: &Self) -> Ordering {
        if self.eq(other) {
            return Ordering::Equal;
        }
        if self.mtime.eq(&other.mtime) {
            return self.path.cmp(&other.path);
        }
        self.mtime.cmp(&other.mtime)
    }
}

impl PartialOrd for FileMeta {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl PartialEq for FileMeta {
    fn eq(&self, other: &Self) -> bool {
        self.mtime == other.mtime && self.path == other.path
    }
}

impl Eq for FileMeta {}
