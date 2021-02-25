//! Utilities for the formatters themselves.
use crate::config::FmtConfig;
use crate::expand_path;
use crate::CLOG;
use anyhow::{anyhow, Result};
use globset::{GlobBuilder, GlobSet, GlobSetBuilder};
use std::{fmt, path::PathBuf};
use std::{
    path::Path,
    process::{Command, Output},
};
use which::which;

/// newtype for the formatter name
#[derive(Debug, Clone, PartialEq, Eq, PartialOrd, Ord)]
pub struct FormatterName(String);

/// Display formatters as "#name"
impl fmt::Display for FormatterName {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "#{}", self.0)
    }
}

/// An instance of a formatter respecting the spec.
#[derive(Debug, Clone)]
pub struct Formatter {
    /// Name of the formatter for display purposes
    pub name: FormatterName,
    /// Command formatter to run
    pub command: PathBuf,
    /// Argument for formatter
    pub options: Vec<String>,
    /// Working directory for formatter
    pub work_dir: PathBuf,
    /// File or Folder that is included to be formatted
    pub includes: GlobSet,
    /// File or Folder that is excluded to be formatted
    pub excludes: GlobSet,
}

impl Formatter {
    /// Run the formatter on the given paths
    // TODO: handle E2BIG
    pub fn fmt<'a>(self: &'a Self, paths: &'a Vec<PathBuf>) -> Result<Output> {
        let mut cmd_arg = Command::new(&self.command.clone());
        // Set the command to run under its working directory.
        cmd_arg.current_dir(&self.work_dir.clone());
        // Append the default options to the command.
        cmd_arg.args(&self.options);
        // Append all of the file paths to format.
        cmd_arg.args(&paths.clone());
        // And run
        Ok(cmd_arg.output()?)
    }

    /// Returns the formatter if the path matches the formatter rules.
    pub fn is_match<T: AsRef<Path>>(self: Self, path: T) -> bool {
        let path = path.as_ref();
        assert!(path.is_absolute());
        // Ignore any paths that are outside of the formatter work_dir
        if !path.starts_with(&self.work_dir) {
            return false;
        }
        // Ignore if any of the excludes match
        if self.excludes.is_match(path) {
            return false;
        }
        // Return true if any of the includes match
        if !self.includes.is_match(path) {
            return false;
        }
        true
    }

    /// Load the formatter matcher from a config fragment
    pub fn from_config(config_dir: &PathBuf, name: &String, cfg: &FmtConfig) -> Result<Self> {
        let name = FormatterName(name.clone());
        // Expand the work_dir to an absolute path, using the config directory as a reference.
        let work_dir = expand_path(&cfg.work_dir, config_dir);
        // Resolve the path to the binary
        let command = which(&cfg.command)?;
        CLOG.debug(&format!(
            "Found {} at {}",
            cfg.command.display(),
            command.display()
        ));
        assert!(command.is_absolute());

        // Build the include and exclude globs
        if cfg.includes.is_empty() {
            return Err(anyhow!("{} doesn't have any includes", name));
        }
        let includes = patterns_to_glob_set(&cfg.includes)?;
        let excludes = patterns_to_glob_set(&cfg.excludes)?;

        Ok(Self {
            name: name,
            command,
            options: cfg.options.clone(),
            work_dir,
            includes,
            excludes,
        })
    }
}

/// Display formatters as "#name"
impl fmt::Display for Formatter {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "#{}", self.name.0)
    }
}

/// Small utility to convert config globs to a GlobSet.
fn patterns_to_glob_set(patterns: &[String]) -> Result<GlobSet> {
    let mut sum = GlobSetBuilder::new();
    for pattern in patterns {
        let glob = GlobBuilder::new(&pattern).build()?;
        sum.add(glob);
    }
    Ok(sum.build()?)
}
