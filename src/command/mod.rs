//! CLI command structures for formatter
#![allow(clippy::redundant_closure)]

mod format;
mod init;

use self::format::format_cmd;
use self::init::init_cmd;
use super::customlog::LogLevel;
use std::path::PathBuf;
use structopt::StructOpt;
use anyhow::{anyhow, Result};

#[derive(Debug, StructOpt)]
/// The various kinds of commands that `treefmt` can execute.
pub enum Command {
    #[structopt(name = "--init")]
    ///  init a new project with a default config
    Init {
        /// path to file or folder
        path: Option<PathBuf>,
    },
    #[structopt(name = "--config")]
    ///  Specify treefmt.toml file
    PrjFmt {
        /// path to file or folder
        path: PathBuf,
    },
}

/// ✨  format all your language!
#[derive(Debug, StructOpt)]
pub struct Cli {
    /// Main command to run
    #[structopt(subcommand)]
    pub cmd: Option<Command>,

    /// Log verbosity is based off the number of v used
    #[structopt(long = "verbose", short = "v", parse(from_occurrences))]
    pub verbosity: u8,

    #[structopt(long = "quiet", short = "q")]
    /// No output printed to stdout
    pub quiet: bool,

    #[structopt(long = "log-level", default_value = "debug")]
    /// The maximum level of messages that should be logged by treefmt. [possible values: info, warn, error]
    pub log_level: LogLevel,
}

/// Run a command with the given logger
pub fn run_cli(cli: Cli) -> anyhow::Result<()> {
    match cli.cmd {
        Some(Command::Init { path }) => init_cmd(path)?,
        Some(Command::PrjFmt { path }) => format_cmd(Some(path))?,
        None => format_cmd(None)?,
    }

    return Ok(());
}

/// Look up treefmt toml from current directory up into project's root
pub fn lookup_treefmt_toml(path: PathBuf) -> Result<PathBuf> {
    let mut work = path;
    loop {
        if work.join("treefmt.toml").exists() {
            return Ok(work);
        }
        let prev = work.clone();
        work = match work.parent() {
            Some(x) => x.to_path_buf(),
            None => return Err(anyhow!("You already reached root directory"))
        };
        if prev == work {
            return Err(anyhow!("treefmt.toml could not be found"))
        }
    }
}
