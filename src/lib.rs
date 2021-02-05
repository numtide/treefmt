//! Your favorite all-in-one formatter tool!

#![deny(missing_docs)]

pub mod command;
pub mod customlog;
pub mod emoji;
pub mod formatters;

use anyhow::anyhow;
use command::run_prjfmt_cli;
use formatters::tool::run_prjfmt;
use std::env;
use std::fs;
use std::path::{Path, PathBuf};
use structopt::StructOpt;

use customlog::{CustomLogOutput, LogLevel};

/// The global custom log and user-facing message output.
pub static CLOG: CustomLogOutput = CustomLogOutput::new();

/// ✨  format all your language!
#[derive(Debug, StructOpt)]
pub struct Cli {
    /// Main command to run
    #[structopt(subcommand)]
    pub cmd: Option<command::Command>,

    /// Log verbosity is based off the number of v used
    #[structopt(long = "verbose", short = "v", parse(from_occurrences))]
    pub verbosity: u8,

    #[structopt(long = "quiet", short = "q")]
    /// No output printed to stdout
    pub quiet: bool,

    #[structopt(long = "log-level", default_value = "info")]
    /// The maximum level of messages that should be logged by prjfmt. [possible values: info, warn, error]
    pub log_level: LogLevel,

    /// Files to format
    pub files: Option<PathBuf>,
}

/// Run a command with the given logger!
pub fn run_cli(cli: Cli) -> anyhow::Result<()> {
    if let Some(command) = cli.cmd {
        run_prjfmt_cli(command)?;
        return Ok(());
    }

    let cwd = match cli.files {
        Some(path) => path,
        None => env::current_dir()?,
    };

    let prjfmt_toml = cwd.as_path().join("prjfmt.toml");
    let xdg_cache_dir = match env::var("XDG_CACHE_DIR") {
        Ok(path) => path,
        Err(err) => {
            CLOG.warn(&format!("{}", err));
            match env::var("HOME") {
                Ok(h) => {
                    let home_cache = Path::new(&h).join(".cache");
                    CLOG.warn(&format!(
                        "Set the $XDG_CACHE_DIR to {}",
                        home_cache.display()
                    ));
                    home_cache.as_path().display().to_string()
                }
                Err(err) => return Err(anyhow!("cannot open HOME due to {}.", err)),
            }
        }
    };

    if prjfmt_toml.as_path().exists() {
        CLOG.info(&format!(
            "Found {} at {}",
            prjfmt_toml.display(),
            cwd.display()
        ));
        CLOG.info(&format!("Change current directory into: {}", cwd.display()));
        let cache_dir = Path::new(&xdg_cache_dir).join("prjfmt/eval-cache");
        fs::create_dir_all(&cache_dir)?;
        run_prjfmt(cwd, cache_dir)?;
    } else {
        println!(
            "file prjfmt.toml couldn't be found. Run `--init` to generate the default setting"
        );
    }
    Ok(())
}
