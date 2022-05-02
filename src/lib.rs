//! Your favorite all-in-one formatter tool!

#![deny(missing_docs)]
pub mod command;
pub mod config;
pub mod customlog;
pub mod engine;
pub mod eval_cache;
pub mod formatter;

use anyhow::Result;
use filetime::FileTime;
use path_clean::PathClean;
use serde::{Deserialize, Serialize};
use std::env;
use std::fs::Metadata;
use std::path::{Path, PathBuf};
use which::which_in;

/// FileMeta represents file meta that change on file modification
/// It currently stores a file mtime and size tuple.
#[derive(Debug, PartialEq, Eq, PartialOrd, Ord, Deserialize, Serialize, Copy, Clone)]
pub struct FileMeta {
    /// File mtime
    pub mtime: i64,
    /// File size
    pub size: u64,
}

/// Small utility that stat() and retrieve the mtime of a file path
pub fn get_path_meta(path: &Path) -> Result<FileMeta> {
    let metadata = std::fs::metadata(path)?;
    Ok(get_meta(&metadata))
}

/// Small utility that stat() and retrieve the mtime of a file metadata
#[must_use]
pub fn get_meta(metadata: &Metadata) -> FileMeta {
    FileMeta {
        mtime: FileTime::from_last_modification_time(metadata).unix_seconds(),
        size: metadata.len(),
    }
}

/// Resolve the command into an absolute path.
fn expand_exe(command: &str, reference: &Path) -> Result<PathBuf> {
    Ok(which_in(command, env::var_os("PATH"), reference)?.clean())
}

/// Returns an absolute path. If the path is absolute already, leave it alone. Otherwise join it to the reference path.
/// Then clean all superfluous ../
#[must_use]
pub fn expand_path(path: &Path, reference: &Path) -> PathBuf {
    let new_path = if path.is_absolute() {
        path.to_path_buf()
    } else {
        reference.join(path)
    };
    new_path.clean()
}

/// Only expands the path if the string contains a slash (/) in it. Otherwise consider it as a string.
pub fn expand_if_path(str: String, reference: &Path) -> String {
    if str.contains('/') {
        expand_path(Path::new(&str), reference)
            .to_string_lossy()
            .to_string()
    } else {
        format!("*/{}", str)
    }
}

#[cfg(test)]
mod tests {
    use super::expand_if_path;
    use globset::{GlobBuilder, GlobSetBuilder};
    use std::path::Path;

    #[test]
    fn test_expand_if_path_single_pattern() {
        let path = vec![
            "/Foo.hs",
            "/nested/Foo.hs",
            "/nested/nested_again/Foo.hs",
            "/different_folder/Foo.hs",
            "/nested/different_folder/Foo.hs",
        ];
        let pattern = "Foo.hs";
        let mut sum = GlobSetBuilder::new();
        let tree_root = Path::new("/");
        let pat = expand_if_path(pattern.to_string(), &tree_root);
        let glob = GlobBuilder::new(&pat).build().unwrap();
        sum.add(glob);
        let result = sum.build().unwrap();
        let test = path
            .clone()
            .into_iter()
            .filter(|p| result.is_match(p))
            .collect::<Vec<&str>>();

        assert_eq!(path, test);
    }

    #[test]
    fn test_expand_if_path_wildcard() {
        let path = vec![
            "/Foo.hs",
            "/nested/Foo.hs",
            "/nested/nested_again/Foo.hs",
            "/different_folder/Foo.hs",
            "/nested/different_folder/Foo.hs",
            "/nested/Bar.hs",
            "/nested/different_folder/Bar.hs",
        ];

        let pattern = "*.hs";
        let mut sum = GlobSetBuilder::new();
        let tree_root = Path::new("/");
        let pat = expand_if_path(pattern.to_string(), &tree_root);
        let glob = GlobBuilder::new(&pat).build().unwrap();
        sum.add(glob);
        let result = sum.build().unwrap();
        let test = path
            .clone()
            .into_iter()
            .filter(|p| result.is_match(p))
            .collect::<Vec<&str>>();

        assert_eq!(path, test);
    }

    #[test]
    fn test_expand_if_path_single_pattern_with_slash() {
        let path = vec![
            "/Foo.hs",
            "/nested/Foo.hs",
            "/nested/nested/Foo.hs",
            "/different_folder/Foo.hs",
            "/nested/different_folder/Foo.hs",
            "/nested/Bar.hs",
            "/nested/different_folder/Bar.hs",
        ];
        let pattern = "nested/Foo.hs";
        let mut sum = GlobSetBuilder::new();
        let tree_root = Path::new("/");
        let pat = expand_if_path(pattern.to_string(), &tree_root);
        let glob = GlobBuilder::new(&pat).build().unwrap();
        sum.add(glob);
        let result = sum.build().unwrap();
        let test = path
            .into_iter()
            .filter(|p| result.is_match(p))
            .collect::<Vec<&str>>();

        assert_eq!(vec!["/nested/Foo.hs"], test);
    }
}
