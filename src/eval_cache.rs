//! Keep track of evaluations
use crate::{
    customlog,
    formatter::{Formatter, FormatterName},
    get_path_mtime, Mtime, CLOG,
};

use anyhow::Result;
use serde::{Deserialize, Serialize};
use sha1::{Digest, Sha1};
use std::fs::{read_to_string, File};
use std::io::Write;
use std::path::{Path, PathBuf};
use std::{borrow::BorrowMut, collections::BTreeMap};

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

#[derive(Debug, Deserialize, Serialize)]
/// RootManifest
pub struct CacheManifest {
    /// Map of all the formatter infos
    pub formatters: BTreeMap<FormatterName, FormatterInfo>,
    /// Map of all the formatted paths
    pub matches: BTreeMap<FormatterName, BTreeMap<PathBuf, Mtime>>,
}

impl Default for CacheManifest {
    fn default() -> Self {
        Self {
            formatters: BTreeMap::new(),
            matches: BTreeMap::new(),
        }
    }
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
        CLOG.debug(&format!("cache: loading from {}", manifest_path.display()));
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
                CLOG.warn(&format!(
                    "cache: failed to load the manifest due to: {}",
                    err
                ));
                Self::default()
            }
        }
    }

    /// Seralizes back the manifest into place.
    pub fn try_write(self, cache_dir: &Path, treefmt_toml: &Path) -> Result<()> {
        let manifest_path = get_manifest_path(cache_dir, treefmt_toml);
        CLOG.debug(&format!("cache: writing to {}", manifest_path.display()));
        let mut f = File::create(manifest_path)?;
        f.write_all(
            format!(
                "# {} DO NOT HAND EDIT THIS FILE {}\n\n{}",
                customlog::WARN,
                customlog::WARN,
                toml::to_string_pretty(&self)?
            )
            .as_bytes(),
        )?;
        Ok(())
    }

    /// Seralizes back the manifest into place.
    pub fn write(self, cache_dir: &Path, treefmt_toml: &Path) {
        if let Err(err) = self.try_write(cache_dir, treefmt_toml) {
            CLOG.warn(&format!("cache: failed to write to disk: {}", err));
        };
    }

    /// Checks and inserts the formatter info into the cache.
    /// If the formatter info has changed, invalidate all the old paths.
    #[must_use]
    pub fn update_formatters(self, formatters: BTreeMap<FormatterName, Formatter>) -> Self {
        let mut new_formatters = BTreeMap::new();
        let mut new_paths = self.matches.clone();
        for (name, fmt) in formatters {
            match load_formatter_info(&fmt) {
                Ok(new_fmt_info) => {
                    if let Some(old_fmt_info) = self.formatters.get(&name) {
                        // Invalidate the old paths if the formatter config has changed.
                        if old_fmt_info != &new_fmt_info {
                            new_paths.remove(&name);
                        }
                    }
                    // Record the new formatter info
                    new_formatters.insert(name, new_fmt_info);
                }
                Err(err) => {
                    // TODO: This probably means that there is a deeper issue with the formatter and
                    //       the formatter will fail down the line.
                    CLOG.error(&format!("cache: failed to load the formatter info {}", err))
                }
            }
        }

        // Now discard all the paths who don't have an associated formatter
        for (name, _) in self.matches {
            if !new_formatters.contains_key(&name) {
                new_paths.remove(&name);
            }
        }

        // Replace with the new info
        Self {
            formatters: new_formatters,
            matches: new_paths,
        }
    }

    /// Returns a new map with all the paths that haven't changed
    #[must_use]
    pub fn filter_matches(
        self,
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
    #[must_use]
    pub fn add_results(self, matches: BTreeMap<FormatterName, BTreeMap<PathBuf, Mtime>>) -> Self {
        // Get a copy of the old matches
        let mut new_matches = self.matches.to_owned();
        // This is really ugly. Get a second copy to work around lifetime issues.
        let mut new_matches_cmp = self.matches.to_owned();

        // Merge all the new matches into it
        for (name, path_infos) in matches {
            let mut def = BTreeMap::new();
            let merged_path_infos = new_matches_cmp
                .get_mut(&name)
                .unwrap_or_else(|| def.borrow_mut());
            for (path, mtime) in path_infos {
                merged_path_infos.insert(path.clone(), mtime);
            }
            new_matches.insert(name, merged_path_infos.to_owned());
        }

        Self {
            formatters: self.formatters,
            matches: new_matches,
        }
    }
}

/// Gets all the info we want from the formatter
fn load_formatter_info(fmt: &Formatter) -> Result<FormatterInfo> {
    let command = fmt.command.clone();
    let command_mtime = get_path_mtime(&command)?;
    // Resolve symlinks and everything
    let command_resolved = std::fs::canonicalize(command.clone())?;
    let command_resolved_mtime = get_path_mtime(&command_resolved)?;
    let options = fmt.options.clone();
    let work_dir = fmt.work_dir.clone();
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
