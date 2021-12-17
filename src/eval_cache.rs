//! Keep track of evaluations
use crate::{
    formatter::{Formatter, FormatterName},
    get_path_mtime, Mtime,
};

use anyhow::Result;
use log::{debug, error, warn};
use serde::{Deserialize, Serialize};
use sha1::{Digest, Sha1};
use std::collections::BTreeMap;
use std::fs::{read_to_string, File};
use std::io::Write;
use std::path::{Path, PathBuf};

/// Metadata about the formatter
#[derive(Debug, Deserialize, Serialize, PartialEq, Eq, Clone)]
pub struct FormatterInfo {
    /// Absolute path to the command
    pub command: PathBuf,
    /// Mtime of the command
    pub command_mtime: Mtime,
    /// Absolute and symlink-resolved path to the command
    pub command_resolved: PathBuf,
    /// mtime of the above
    pub command_resolved_mtime: Mtime,
    /// formatter options
    pub options: Vec<String>,
    /// work_dir
    pub work_dir: PathBuf,
}

#[derive(Debug, Default, Deserialize, Serialize)]
/// RootManifest
pub struct CacheManifest {
    /// Map of all the formatter infos
    pub formatters: BTreeMap<FormatterName, FormatterInfo>,
    /// Map of all the formatted paths
    pub matches: BTreeMap<FormatterName, BTreeMap<PathBuf, Mtime>>,
}

impl Clone for CacheManifest {
    fn clone(&self) -> Self {
        Self {
            formatters: self.formatters.clone(),
            matches: self.matches.clone(),
        }
    }
}

impl CacheManifest {
    /// Loads the manifest and returns an error if it failed
    pub fn try_load(cache_dir: &Path, treefmt_toml: &Path) -> Result<Self> {
        let manifest_path = get_manifest_path(cache_dir, treefmt_toml);
        debug!("cache: loading from {}", manifest_path.display());
        let content = read_to_string(&manifest_path)?;
        let manifest = toml::from_str(&content)?;
        Ok(manifest)
    }

    /// Always loads the manifest. If an error occured, log and return an empty manifest.
    #[must_use]
    pub fn load(cache_dir: &Path, treefmt_toml: &Path) -> Self {
        match Self::try_load(cache_dir, treefmt_toml) {
            Ok(manifest) => manifest,
            Err(err) => {
                warn!("cache: failed to load the manifest due to: {}", err);
                Self::default()
            }
        }
    }

    /// Seralizes back the manifest into place.
    pub fn try_write(self, cache_dir: &Path, treefmt_toml: &Path) -> Result<()> {
        let manifest_path = get_manifest_path(cache_dir, treefmt_toml);
        debug!("cache: writing to {}", manifest_path.display());
        let mut f = File::create(manifest_path)?;
        f.write_all(
            format!(
                "# DO NOT HAND EDIT THIS FILE\n\n{}",
                toml::to_string_pretty(&self)?
            )
            .as_bytes(),
        )?;
        Ok(())
    }

    /// Seralizes back the manifest into place.
    pub fn write(self, cache_dir: &Path, treefmt_toml: &Path) {
        if let Err(err) = self.try_write(cache_dir, treefmt_toml) {
            warn!("cache: failed to write to disk: {}", err);
        };
    }

    /// Checks and inserts the formatter info into the cache.
    /// If the formatter info has changed, invalidate all the old paths.
    pub fn update_formatters(&mut self, formatters: BTreeMap<FormatterName, Formatter>) {
        let mut old_formatters = std::mem::take(&mut self.formatters);
        for (name, fmt) in formatters {
            match load_formatter_info(fmt) {
                Ok(new_fmt_info) => {
                    if let Some(old_fmt_info) = old_formatters.remove(&name) {
                        // Invalidate the old paths if the formatter config has changed.
                        if old_fmt_info != new_fmt_info {
                            self.matches.remove(&name);
                        }
                    }
                    // Record the new formatter info
                    self.formatters.insert(name, new_fmt_info);
                }
                Err(err) => {
                    // TODO: This probably means that there is a deeper issue with the formatter and
                    //       the formatter will fail down the line.
                    error!("cache: failed to load the formatter info {}", err)
                }
            }
        }

        // Now discard all the paths who don't have an associated formatter
        for name in old_formatters.keys() {
            self.matches.remove(name);
        }
    }

    /// Returns a new map with all the paths that haven't changed
    #[must_use]
    pub fn filter_matches(
        &self,
        matches: BTreeMap<FormatterName, BTreeMap<PathBuf, Mtime>>,
    ) -> BTreeMap<FormatterName, BTreeMap<PathBuf, Mtime>> {
        matches
            .into_iter()
            .fold(BTreeMap::new(), |mut sum, (key, path_infos)| {
                let new_path_infos = match self.matches.get(&key) {
                    Some(prev_paths) => {
                        path_infos
                            .into_iter()
                            .fold(BTreeMap::new(), |mut sum, (path, mtime)| {
                                // Mtime(-1) is not a valid mtime and will therefor never match
                                let prev_mtime = prev_paths.get(&path).unwrap_or(&Mtime(-1));
                                if prev_mtime != &mtime {
                                    // Keep the path if the mtimes don't match
                                    sum.insert(path, mtime);
                                }
                                sum
                            })
                    }
                    None => path_infos,
                };
                sum.insert(key, new_path_infos);
                sum
            })
    }

    /// Merge recursively the new matches with the existing entries in the cache
    pub fn add_results(&mut self, matches: BTreeMap<FormatterName, BTreeMap<PathBuf, Mtime>>) {
        for (name, path_infos) in matches {
            if let Some(old_path_infos) = self.matches.get_mut(&name) {
                old_path_infos.extend(path_infos);
            } else {
                self.matches.insert(name, path_infos);
            }
        }
    }
}

/// Gets all the info we want from the formatter
fn load_formatter_info(fmt: Formatter) -> Result<FormatterInfo> {
    let command = fmt.command;
    let command_mtime = get_path_mtime(&command)?;
    // Resolve symlinks and everything
    let command_resolved = std::fs::canonicalize(command.clone())?;
    let command_resolved_mtime = get_path_mtime(&command_resolved)?;
    let options = fmt.options;
    let work_dir = fmt.work_dir;
    // TODO: does it matter if the includes and excludes are missing?
    Ok(FormatterInfo {
        command,
        command_mtime,
        command_resolved,
        command_resolved_mtime,
        options,
        work_dir,
    })
}

/// Derive the manifest filename from the treefmt_toml path.
fn get_manifest_path(cache_dir: &Path, treefmt_toml: &Path) -> PathBuf {
    assert!(cache_dir.is_absolute());
    assert!(treefmt_toml.is_absolute());
    // FIXME: it's a shame that we can't access the underlying OsStr bytes
    let path_bytes = treefmt_toml.to_string_lossy();
    // Hash the config path
    let treefmt_hash = Sha1::digest(path_bytes.as_bytes());
    // Hexencode
    let filename = format!("{:x}.toml", treefmt_hash);
    cache_dir.join(filename)
}
