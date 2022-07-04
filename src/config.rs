//! Contains the project configuration schema and parsing
use anyhow::Result;
use serde::Deserialize;
use std::fs::read_to_string;
use std::path::PathBuf;
use std::{collections::BTreeMap, path::Path};

/// Name of the config file
pub const FILENAME: &str = "treefmt.toml";
/// Alternative name of the config file
pub const FILENAMEALT: &str = ".treefmt.toml";

/// treefmt.toml structure
#[derive(Debug, Deserialize)]
pub struct Root {
    /// Config that applies to every formatter
    pub global: Option<GlobalConfig>,
    /// Map of formatters into the config
    pub formatter: BTreeMap<String, FmtConfig>,
}

/// Global config which applies to every formatter
#[derive(Debug, Deserialize)]
pub struct GlobalConfig {
    /// Global glob to exclude files or folder for all formatters
    #[serde(default)]
    pub excludes: Vec<String>,
}

/// Config for each formatters
#[derive(Debug, Deserialize)]
pub struct FmtConfig {
    /// Command formatter to run
    pub command: String,
    /// Working directory for formatter
    #[serde(default = "cwd")]
    pub work_dir: PathBuf,
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

// The default work_dir value. It's a bit clunky. See https://github.com/serde-rs/serde/issues/1814
fn cwd() -> PathBuf {
    ".".into()
}

/// Returns an absolute path to the treefmt.toml file. From the current folder, and up.
#[must_use]
pub fn lookup(dir: &Path) -> Option<PathBuf> {
    let mut cwd = dir.to_path_buf();
    loop {
        let config_file = cwd.join(FILENAME);
        let config_file_alt = cwd.join(FILENAMEALT);
        if config_file.exists() {
            return Some(config_file);
        } else if config_file_alt.exists() {
            return Some(config_file_alt);
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
pub fn from_path(file_path: &Path) -> Result<Root> {
    // Load the file
    let content = read_to_string(file_path)?;
    // Parse the config
    let ret: Root = toml::from_str(&content)?;
    // Expand a bunch of formatter configs. If any of these fail, don't make it a fatal issue. Display the error and continue.
    Ok(ret)
}
