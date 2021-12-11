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
use std::path::PathBuf;
use structopt::StructOpt;

/// ✨  format all your language!
#[derive(Debug, StructOpt)]
pub struct Cli {
    /// Create a new treefmt.toml
    #[structopt(long = "init")]
    pub init: bool,

    /// Format the content passed in stdin
    #[structopt(long = "stdin", conflicts_with("init"))]
    pub stdin: bool,

    /// Clear the evaluation cache. Use in case the cache is not precise enough.
    #[structopt(long = "clear-cache", conflicts_with("stdin"), conflicts_with("init"))]
    pub clear_cache: bool,

    /// Exit with error if any changes were made. Useful for CI.
    #[structopt(
        long = "fail-on-change",
        conflicts_with("stdin"),
        conflicts_with("init")
    )]
    pub fail_on_change: bool,

    /// Log verbosity is based off the number of v used
    #[structopt(long = "verbose", short = "v", parse(from_occurrences))]
    pub verbosity: u8,

    #[structopt(long = "quiet", short = "q")]
    /// No output printed to stderr
    pub quiet: bool,

    #[structopt(short = "C", default_value = ".")]
    /// Run as if treefmt was started in <work-dir> instead of the current working directory.
    pub work_dir: PathBuf,

    #[structopt(long = "tree-root")]
    /// Set the path to the tree root directory. Defaults to the folder holding the treefmt.toml file.
    pub tree_root: Option<PathBuf>,

    #[structopt(long = "config-file")]
    /// Run with the specified config file, which is not required to be in the tree to be formatted.
    pub config_file: Option<PathBuf>,

    #[structopt()]
    /// Paths to format. Defaults to formatting the whole tree.
    pub paths: Vec<PathBuf>,
}

/// Use this instead of Cli::from_args(). We do a little bit of post-processing here.
pub fn cli_from_args() -> anyhow::Result<Cli> {
    let mut cli = Cli::from_args();
    let cwd = std::env::current_dir()?;
    assert!(cwd.is_absolute());
    // Make sure the work_dir is an absolute path. Don't use the stdlib canonicalize() function
    // because symlinks should not be resolved.
    cli.work_dir = expand_path(&cli.work_dir, &cwd);

    // Make sure the tree_root is an absolute path.
    if let Some(tree_root) = cli.tree_root {
        cli.tree_root = Some(expand_path(&tree_root, &cwd));
    }

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

    // Make sure the config_file points to an absolute path.
    if let Some(config_file) = cli.config_file {
        cli.config_file = Some(expand_path(&config_file, &cwd));
    }

    Ok(cli)
}

/// Run a command with the given logger
pub fn run_cli(cli: &Cli) -> anyhow::Result<()> {
    if cli.init {
        init_cmd(&cli.work_dir)?
    } else if cli.stdin {
        format_stdin_cmd(&cli.tree_root, &cli.work_dir, &cli.paths)?
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
            cli.clear_cache,
            cli.fail_on_change,
        )?
    }

    Ok(())
}
