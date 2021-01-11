use crate::emoji;
use console::style;

// use super::tools;
use anyhow::{anyhow, Result};
use glob::{glob, Paths};
use serde::Deserialize;
use std::collections::BTreeMap;
use std::iter::Iterator;
use std::path::PathBuf;
use std::vec::IntoIter;
// use tools::{
//     go::{deserialize_go, Gfmt},
//     haskell::{deserialize_haskell, Hfmt},
//     rust::{deserialize_rust, Rfmt},
// };
use xshell::cmd;

// TODO: This module provides functions to install all formatter
// that is not available in user's $PATH
/// Make sure that rustfmt exists. This also for other formatter
pub fn check_fmt(command: String) -> Result<()> {
    let cmd_bin = command.split_ascii_whitespace().next().unwrap_or("");
    if let Ok(str) = cmd!("which {cmd_bin}").read() {
        return Ok(());
    }
    anyhow::bail!(
        "Failed to locate formatter named {}. \
        Please make sure it is available in your PATH",
        command
    )
}

/// Running the fmt
pub fn run_fmt(cmd_arg: &str, args: &Vec<String>, path: PathBuf) -> anyhow::Result<()> {
    cmd!("{cmd_arg} {args...} {path}").run()?;
    Ok(())
}

/// Convert glob pattern into list of pathBuf
pub fn glob_to_path(cwd: PathBuf, extensions: FileExtensions) -> Result<Paths> {
    match extensions {
        FileExtensions::SingleFile(sfile) => {
            let dir = cwd.as_path().to_str().unwrap_or("");
            let pat = format!("{}**/{}", dir, &sfile);
            match glob(&pat) {
                Ok(paths) => Ok(paths),
                Err(err) => {
                    anyhow::bail!(
                        "{} Error at position: {} due to {}",
                        emoji::ERROR,
                        err.pos,
                        err.msg
                    )
                }
            }
        }
        FileExtensions::MultipleFile(strs) => {
            let files = strs
                .into_iter()
                .map(|str| {
                    let dir = cwd.as_path().to_str().unwrap_or("");
                    let pat = format!("{}**/{}", dir, &str);
                    match glob(&pat) {
                        Ok(paths) => Ok(paths),
                        Err(err) => {
                            anyhow::bail!(
                                "{} Error at position: {} due to {}",
                                emoji::ERROR,
                                err.pos,
                                err.msg
                            )
                        }
                    }
                })
                .flatten()
                .nth(0);
            match files {
                Some(paths) => Ok(paths),
                None => {
                    anyhow::bail!("{} Blob not found", emoji::ERROR)
                }
            }
        }
    }
}

/// Convert glob's Paths into list of PathBuf
pub fn paths_to_pathbuf(
    inc: Vec<String>,
    excl: Vec<String>,
    paths_list: Vec<Paths>,
) -> Vec<PathBuf> {
    Vec::new()
}

/// fmt.toml structure
#[derive(Debug, Deserialize)]
pub struct Root {
    /// Map of formatters into the config
    pub formatters: BTreeMap<String, FmtConfig>,
}

/// Config for each formatters
#[derive(Debug, Deserialize)]
pub struct FmtConfig {
    /// File extensions that want to be formatted
    pub files: FileExtensions,
    /// File or Folder that is included to be formatted
    pub includes: Option<Vec<String>>,
    /// File or Folder that is excluded to be formatted
    pub excludes: Option<Vec<String>>,
    /// Command formatter to run
    pub command: Option<String>,
    /// Argument for formatter
    pub args: Option<Vec<String>>,
}

/// File extensions can be single string (e.g. "*.hs") or
/// list of string (e.g. [ "*.hs", "*.rs" ])
#[derive(Debug, Deserialize, Clone)]
#[serde(untagged)]
pub enum FileExtensions {
    /// Single file type
    SingleFile(String),
    /// List of file type
    MultipleFile(Vec<String>),
}
