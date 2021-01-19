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
extern crate hex;
extern crate log;
extern crate serde;
extern crate sha1;
extern crate toml;
extern crate walkdir;

pub mod command;
pub mod customlog;
pub mod emoji;
pub mod formatters;

use command::run_prjfmt_cli;
use formatters::check::check_prjfmt;
use formatters::manifest::{read_prjfmt_manifest, RootManifest};
use formatters::tool::run_prjfmt;
use std::env;
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
            CLOG.warn(&format!("Set the $XDG_CACHE_DIR to {}", cwd.display()));
            cwd.as_path().display().to_string()
        }
    };

    if let true = prjfmt_toml.as_path().exists() {
        CLOG.info(&format!("Found {} at {}", prjfmt_toml.display(), cwd.display()));
        CLOG.info(&format!("Change current directory into: {}", cwd.display()));
        let cache_dir = Path::new(&xdg_cache_dir).join("prjfmt/eval-cache");

        if let true = cache_dir.as_path().exists() {
            // Once the prjfmt found the $XDG_CACHE_DIR/prjfmt/eval-cache/ folder,
            // it will try to scan the manifest and passed it into check_prjfmt function
            let manifest: RootManifest = read_prjfmt_manifest(&prjfmt_toml, &cache_dir)?;
            check_prjfmt(prjfmt_toml, &manifest)?;
            run_prjfmt(cwd, cache_dir)?;
        } else {
            // If prjfmt cannot find the $XDG_CACHE_DIR/prjfmt/eval-cache/, then it assumes
            // that all files are never get formatted.
            println!("First time formatting with prjfmt.");
            run_prjfmt(cwd, cache_dir)?;
        }
    } else {
        println!(
            "file prjfmt.toml couldn't be found. Run `--init` to generate the default setting"
        );
    }
    Ok(())
}
