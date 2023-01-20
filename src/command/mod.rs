//! CLI command structures for formatter
#![allow(clippy::redundant_closure)]

mod format;
mod format_stdin;
mod init;

use self::format::format_cmd;
use self::format_stdin::format_stdin_cmd;
use self::init::init_cmd;
use crate::config;
use crate::expand_path;
use anyhow::anyhow;
use clap::Parser;
use clap_verbosity_flag::Verbosity;
use std::{
    env,
    path::{Path, PathBuf},
};

/// ✨  format all your language!
#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
pub struct Cli {
    /// Create a new treefmt.toml
    #[arg(short, long, default_value_t = false)]
    pub init: bool,

    /// Format the content passed in stdin.
    #[arg(long, default_value_t = false, conflicts_with("init"))]
    pub stdin: bool,

    /// Ignore the evaluation cache entirely. Useful for CI.
    #[arg(long, conflicts_with("stdin"), conflicts_with("init"))]
    pub no_cache: bool,

    /// Reset the evaluation cache. Use in case the cache is not precise enough.
    #[arg(short, long, default_value_t = false)]
    pub clear_cache: bool,

    /// Exit with error if any changes were made. Useful for CI.
    #[arg(
        long,
        default_value_t = false,
        conflicts_with("stdin"),
        conflicts_with("init")
    )]
    pub fail_on_change: bool,

    /// Do not exit with error if a configured formatter is missing.
    #[arg(long, default_value_t = false)]
    pub allow_missing_formatter: bool,

    /// Log verbosity is based off the number of v used
    #[clap(flatten)]
    pub verbose: Verbosity,

    /// Run as if treefmt was started in <work-dir> instead of the current working directory.
    #[arg(short = 'C', default_value = ".", value_parser = parse_path)]
    pub work_dir: PathBuf,

    /// Set the path to the tree root directory. Defaults to the folder holding the treefmt.toml file.
    #[arg(long, env = "PRJ_ROOT", default_value = ".", value_parser = parse_path)]
    pub tree_root: Option<PathBuf>,

    /// Run with the specified config file, which is not required to be in the tree to be formatted.
    #[arg(long, value_parser = parse_path)]
    pub config_file: Option<PathBuf>,

    /// Paths to format. Defaults to formatting the whole tree.
    #[arg()]
    pub paths: Vec<PathBuf>,

    /// Select formatters name to apply. Defaults to all formatters.
    #[arg(short, long)]
    pub formatters: Option<Vec<String>>,
}

fn parse_path(s: &str) -> anyhow::Result<PathBuf> {
    // Obtain current dir and ensure is absolute
    let cwd = match env::current_dir() {
        Ok(dir) => dir,
        Err(err) => return Err(anyhow!("{}", err)),
    };
    assert!(cwd.is_absolute());

    // TODO: Include validation for incorrect paths or caracters
    let path = Path::new(s);

    // Make sure the path is an absolute path.
    // Don't use the stdlib canonicalize() function
    // because symlinks should not be resolved.
    Ok(expand_path(path, &cwd))
}

/// Use this instead of Cli::parse(). We do a little bit of post-processing here.
pub fn cli_from_args() -> anyhow::Result<Cli> {
    let mut cli = Cli::parse();

    // Check we can find config first before proceeding
    match cli.config_file {
        None => {
            // Find the config file if not specified by the user.
            cli.config_file = config::lookup(&cli.work_dir);
        }
        Some(_) => {
            if cli.tree_root.is_none() {
                return Err(anyhow!(
                    "If --config-file is set, --tree-root must also be set"
                ));
            }
        }
    }

    Ok(cli)
}

/// Run a command with the given logger
pub fn run_cli(cli: &Cli) -> anyhow::Result<()> {
    if cli.init {
        init_cmd(&cli.work_dir)?
    } else if cli.stdin {
        format_stdin_cmd(&cli.tree_root, &cli.work_dir, &cli.paths, &cli.formatters)?
    } else {
        // Fail if configuration could not be found. This is checked
        // here to avoid aborting before init_cmd.
        if cli.config_file.is_none() {
            return Err(anyhow!(
                "{} could not be found in {} and up. Use the --init option to create one or specify --config-file if it is in a non-standard location.",
                config::FILENAME,
                cli.work_dir.display(),
            ));
        }

        format_cmd(
            &cli.tree_root,
            &cli.work_dir,
            cli.config_file
                .as_ref()
                .expect("presence asserted in ::cli_from_args"),
            &cli.paths,
            cli.no_cache,
            cli.clear_cache,
            cli.fail_on_change,
            cli.allow_missing_formatter,
            &cli.formatters,
        )?
    }

    Ok(())
}
