//! CLI command structures for formatter
#![allow(clippy::redundant_closure)]

mod init;

use self::init::init_prjfmt;
use std::path::PathBuf;
use structopt::StructOpt;

#[derive(Debug, StructOpt)]
/// The various kinds of commands that `prjfmt` can execute.
pub enum Command {
    #[structopt(name = "--init")]
    ///  init a new project with a default config
    Init {
        /// path to file or folder
        path: Option<PathBuf>,
    },
}

/// Run a command with the given logger!
pub fn run_prjfmt_cli(command: Command) -> anyhow::Result<()> {
    match command {
        Command::Init { path } => init_prjfmt(path),
    }
}
