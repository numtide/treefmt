//! Functionality related to installing prebuilt binaries

#![deny(missing_docs)]

use std::path::PathBuf;
/// Specific language formatting tools
pub mod tools;
mod tool;
use self::tool::Tools;
use serde::Deserialize;
use std::collections::HashMap;
use xshell::{cmd, pushd, pushenv};

// TODO: This module provides functions to install all formatter
// that is not available in user's $PATH
/// Make sure that rustfmt exists. This also for other formatter
fn check_rustfmt() -> anyhow::Result<()> {
    let out = cmd!("rustfmt --version").read()?;
    if !out.contains("stable") && !out.contains("nightly") {
        anyhow::bail!(
            "Failed to run rustfmt from toolchain 'stable'. \
             Please make sure it is available in your PATH",
        )
    }
    Ok(())
}

/// Running rustfmt
pub fn run_rustfmt(mode: Mode, path: PathBuf) -> anyhow::Result<()> {
    let _dir = pushd(path)?;
    let _e = pushenv("RUSTUP_TOOLCHAIN", "stable");
    check_rustfmt()?;
    let check = match mode {
        Mode::Overwrite => &[][..],
        Mode::Verify => &["--", "--check"],
    };
    cmd!("cargo fmt {check...}").run()?;
    Ok(())
}

/// Formatter mode
#[derive(Debug, PartialEq, Eq, Clone, Copy)]
pub enum Mode {
    /// Start the formatting process
    Overwrite,
    /// Trying to check the project using formatter. This maybe helpful to --dry-run
    Verify,
}

/// fmt.toml structure
#[derive(Debug, Deserialize)]
pub struct Root {
    formatters: HashMap<String, FmtConfig>,
}

/// Config for each formatters
#[derive(Debug, Deserialize)]
struct FmtConfig {
    files: FileExtensions,
    includes: Option<Vec<String>>,
    excludes: Option<Vec<String>>,
    command: Option<Tools>,
    args: Option<Vec<String>>,
}

#[derive(Debug, Deserialize)]
#[serde(untagged)]
enum FileExtensions {
    SingleFile(String),
    MultipleFile(Vec<String>),
}
