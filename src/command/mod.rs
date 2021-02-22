//! CLI command structures for formatter
#![allow(clippy::redundant_closure)]

mod format;
mod init;

use self::format::format_cmd;
use self::init::init_cmd;
use super::customlog::LogLevel;
use std::path::PathBuf;
use structopt::StructOpt;

#[derive(Debug, StructOpt)]
/// The various kinds of commands that `treefmt` can execute.
pub enum Command {
    #[structopt(name = "--init")]
    /// Init a new project with a default config
    Init {},
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

    #[structopt(short = "C")]
    /// Run as if treefmt was started in <path> instead of the current working directory.
    pub work_dir: Option<PathBuf>,
}

/// Run a command with the given logger
pub fn run_cli(cli: Cli) -> anyhow::Result<()> {
    match cli.cmd {
        Some(Command::Init {}) => init_cmd(cli.work_dir)?,
        None => format_cmd(cli.work_dir)?,
    }

    Ok(())
}
