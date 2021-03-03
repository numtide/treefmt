//! CLI command structures for formatter
#![allow(clippy::redundant_closure)]

mod format;
mod init;

use self::format::format_cmd;
use self::init::init_cmd;
use crate::expand_path;
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
    /// Create a new treefmt.toml
    #[structopt(long = "init")]
    pub init: bool,

    /// Clear the evaluation cache. Use in case the cache is not precise enough.
    #[structopt(long = "clear-cache")]
    pub clear_cache: bool,

    /// Exit with error if any changes were made. Useful for CI.
    #[structopt(long = "fail-on-change")]
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
    Ok(cli)
}

/// Run a command with the given logger
pub fn run_cli(cli: &Cli) -> anyhow::Result<()> {
    if cli.init {
        init_cmd(&cli.work_dir)?
    } else {
        format_cmd(
            &cli.tree_root,
            &cli.work_dir,
            &cli.paths,
            cli.clear_cache,
            cli.fail_on_change,
        )?
    }

    Ok(())
}
