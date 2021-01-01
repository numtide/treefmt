//! CLI command structures for formatter
#![allow(clippy::redundant_closure)]

mod init;

use self::init::init_fmt;

use log::info;

#[derive(Debug)]
struct Initialize {}

#[derive(Debug, StructOpt)]
/// The various kinds of commands that `allfmt` can execute.
pub enum Command {
    #[structopt(name = "--init")]
    ///  init a new project with a default config
    Init,
    #[structopt(name = "--dry-run")]
    /// creating formating plan
    Dry,
}

/// Run a command with the given logger!
pub fn run_all_fmt(command: Command) -> anyhow::Result<()> {
    match command {
        Command::Init => {
            info!("creating fmt.toml");
            init_fmt()
        }
        Command::Dry => {
            println!("running dry-run");
            Ok(())
        }
    }
}
