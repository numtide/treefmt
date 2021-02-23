//! Contains the project configuration schema and parsing
use crate::{expand_path, CLOG};
use anyhow::Result;
use serde::Deserialize;
use std::collections::BTreeMap;
use std::fs::read_to_string;
use std::path::PathBuf;
use which::which;

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
    pub command: PathBuf,
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
pub fn lookup(dir: &PathBuf) -> Option<PathBuf> {
    let mut cwd = dir.clone();
    loop {
        let config_file = cwd.join(FILENAME);
        if config_file.exists() {
            return Some(config_file);
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
pub fn from_path(file_path: &PathBuf) -> Result<Root> {
    // unwrap: assume the file is in a folder
    let file_dir = file_path.parent().unwrap();
    // Load the file
    let content = read_to_string(file_path)?;
    // Parse the config
    let mut ret: Root = toml::from_str(&content)?;
    // Expand a bunch of formatter configs. If any of these fail, don't make it a fatal issue. Display the error and continue.
    let new_formatter = ret
        .formatter
        .iter()
        .fold(BTreeMap::new(), |mut sum, (name, fmt)| {
            match load_formatter(fmt, &file_dir.to_path_buf()) {
                // Re-add the resolved formatter if it was successful
                Ok(fmt2) => {
                    sum.insert(name.clone(), fmt2);
                }
                Err(err) => CLOG.warn(&format!("Ignoring {} because of error: {}", name, err)),
            };
            sum
        });
    // Replace the formatters with the loaded ones
    ret.formatter = new_formatter;

    Ok(ret)
}

fn load_formatter(fmt: &FmtConfig, config_dir: &PathBuf) -> Result<FmtConfig> {
    // Expand the work_dir to an absolute path, using the config directory as a reference.
    let abs_work_dir = expand_path(&fmt.work_dir, config_dir);
    // Resolve the path to the binary
    let abs_command = which(&fmt.command)?;
    assert!(abs_command.is_absolute());
    CLOG.debug(&format!(
        "Found {} at {}",
        fmt.command.display(),
        abs_command.display()
    ));
    Ok(FmtConfig {
        command: abs_command,
        work_dir: abs_work_dir,
        options: fmt.options.clone(),
        includes: fmt.includes.clone(),
        excludes: fmt.excludes.clone(),
    })
}
