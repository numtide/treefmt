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
extern crate serde;
extern crate toml;
extern crate walkdir;

pub mod command;
pub mod customlog;
pub mod emoji;
pub mod formatters;

use command::run_all_fmt;
use formatters::tool::{check_fmt, glob_to_path, run_fmt, Root};
use glob::Paths;
use std::fs::read_to_string;
use std::path::PathBuf;
use std::vec;

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

    let cwd = match cli.files {
        Some(path) => path,
        None => PathBuf::from("."),
    };

    let fmt_toml = cwd.as_path().join("fmt.toml");

    if let true = fmt_toml.as_path().exists() {
        println!("found fmt.toml");
        println!("set path to {}", cwd.to_str().unwrap_or(""));
        let open_file = match read_to_string(fmt_toml.as_path()) {
            Ok(file) => file,
            Err(err) => return Err(anyhow::Error::new(err)),
        };
        let toml_content: Root = toml::from_str(&open_file)?;

        let commands = toml_content
            .formatters
            .values()
            .map(|c| c.command.clone().unwrap_or("".into()));

        let args = toml_content.formatters.values().map(|c| {
            let arg = match c.args.clone() {
                Some(vstr) => vstr,
                None => Vec::new(),
            };
            arg
        });

        let all_files = toml_content
            .formatters
            .values()
            .map(|f| f.files.clone())
            .filter_map(|glob| glob_to_path(cwd.clone(), glob).ok());

        for cmd in commands.clone() {
            check_fmt(cmd)?;
        }

        let cmd_args = commands.zip(args).zip(all_files);

        // TODO: implement `filtered_files: Vec<Paths>` for `[includes]` and `[exclude]`
        println!("===========================");
        for ((cmd, args), paths) in cmd_args.clone() {
            println!("Command: {}", cmd);
            println!("Files:");
            for f in paths {
                println!(" - {}", f?.display());
            }
            println!("===========================");
        }

        for ((cmd, args), paths) in cmd_args.clone() {
            for f in paths {
                let path = f?;
                run_fmt(&cmd.clone(), &args, path)?
            }
        }

        println!("Format successful");

        // .and_then(|config| config.includes).unwrap_or(Vec::new());
        // for dir in rustdir {
        //     cwd.push(Path::new(&dir));
        //     run_rustfmt(Mode::Overwrite, cwd.clone())?;
        // }
    } else {
        println!("file fmt.toml couldn't be found. Run `--init` to generate the default setting");
    }

    Ok(())
}
