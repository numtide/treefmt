//! Your favorite all-in-one formatter tool!

#![deny(missing_docs)]

extern crate anyhow;
extern crate console;
extern crate glob;
extern crate parking_lot;
extern crate which;
#[macro_use]
extern crate structopt;
extern crate chrono;
extern crate curl;
extern crate dialoguer;
extern crate log;
extern crate toml;
extern crate walkdir;

pub mod command;
pub mod customlog;
pub mod emoji;
pub mod install;
// pub mod license;
// pub mod lockfile;

use command::run_all_fmt;
use std::path::{Path, PathBuf};

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
    /// The maximum level of messages that should be logged by allfmt. [possible values: info, warn, error]
    pub log_level: LogLevel,

    /// Files to format
    pub files: Option<PathBuf>,
}

/// Run a command with the given logger!
pub fn run_cli(cli: Cli) -> anyhow::Result<()> {
    if let Some(command) = cli.cmd {
        run_all_fmt(command)?;
        return Ok(());
    }

    let mut cwd = match cli.files {
        Some(path) => path,
        None => PathBuf::from("."),
    };

    let fmtToml = cwd.as_path().join("fmt.toml");

    if let true = fmtToml.as_path().exists() {
        println!("found fmt.toml");
        println!("set path to {}/", cwd.to_str().unwrap_or(""));
    } else {
        println!("file fmt.toml couldn't be found. Run `--init` to generate the default setting");
    }

    Ok(())
}
