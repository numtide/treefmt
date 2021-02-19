//! Contains the project configuration schema and parsing
use anyhow::Result;
use serde::Deserialize;
use std::collections::BTreeMap;
use std::fs::read_to_string;
use std::path::PathBuf;

/// Name of the config file
pub const FILENAME: &str = "treefmt.toml";

/// treefmt.toml structure
#[derive(Debug, Deserialize)]
pub struct Root {
    /// Map of formatters into the config
    pub formatter: BTreeMap<String, FmtConfig>,
}

/// Config for each formatters
#[derive(Debug, Deserialize)]
pub struct FmtConfig {
    /// Command formatter to run
    pub command: String,
    /// Working directory for formatter
    pub work_dir: Option<String>,
    /// Argument for formatter
    #[serde(default)]
    pub options: Vec<String>,
    /// File or Folder that is included to be formatted
    #[serde(default)]
    pub includes: Vec<String>,
    /// File or Folder that is excluded to be formatted
    #[serde(default)]
    pub excludes: Vec<String>,
}

/// Find the directory that contains the treefmt.toml file. From the current folder, and up.
pub fn lookup_dir(dir: &PathBuf) -> Option<PathBuf> {
    let mut cwd = dir.clone();
    loop {
        if cwd.join(FILENAME).exists() {
            return Some(cwd);
        }
        cwd = match cwd.parent() {
            Some(x) => x.to_path_buf(),
            // None is returned when .parent() is already the root folder. In that case we have
            // exhausted the search space.
            None => return None,
        };
    }
}

/// Loads the treefmt.toml config from the given file path.
pub fn from_path(path: &PathBuf) -> Result<Root> {
    let content = read_to_string(path)?;
    let ret: Root = toml::from_str(&content)?;
    Ok(ret)
}
