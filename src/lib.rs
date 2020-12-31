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
extern crate serde_derive;

pub mod command;
pub mod customlog;
pub mod emoji;
pub mod install;
// pub mod license;
// pub mod lockfile;

use command::run_all_fmt;
use serde_derive::Deserialize;
use std::env;
use std::fs::{read_to_string};
use std::path::{Path, PathBuf};
use xshell::{cmd, pushd, pushenv};

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

    let fmt_toml = cwd.clone().as_path().join("fmt.toml");

    if let true = fmt_toml.as_path().exists() {
        println!("found fmt.toml");
        println!("set path to {}/", cwd.to_str().unwrap_or(""));
        let open_file = match read_to_string(fmt_toml.as_path()) {
            Ok(file) => file,
            Err(err) => return Err(anyhow::Error::new(err))
        };
        let toml_content: Root = toml::from_str(&open_file)?;
        let rustdir = toml_content.rustfmt
        .and_then(|config| config.includes).unwrap_or(Vec::new());

        for dir in rustdir {
            cwd.push(Path::new(&dir));
            run_rustfmt(Mode::Overwrite, cwd.clone())?;
        }
    } else {
        println!("file fmt.toml couldn't be found. Run `--init` to generate the default setting");
    }

    Ok(())
}

/// Make sure that rustfmt exists. This also for other formatter
fn check_rustfmt() -> anyhow::Result<()> {
    let out = cmd!("rustfmt --version").read()?;
    if !out.contains("stable") && !out.contains("nightly"){
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
struct Root {
    ormolu: Option<FmtConfig>,
    rustfmt: Option<FmtConfig>
}

/// Config for each formatters
#[derive(Debug, Deserialize)]
struct FmtConfig {
    includes: Option<Vec<String>>,
    excludes: Option<Vec<String>>,
    command: Option<String>,
    args: Option<Vec<String>>
}
