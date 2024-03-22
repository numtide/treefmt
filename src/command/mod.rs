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
use clap_verbosity_flag::{InfoLevel, Verbosity};
use log::warn;
use std::{
    env,
    path::{Path, PathBuf},
};

/// Enumeration specifying filesystem scan method for modified files.
#[derive(Debug, Clone)]
pub enum FSScan {
    /// Recursive walking the filesystem using getdents(2) and statx(2) system calls.
    ///
    /// Scales linearly with total count of files in the repository.
    Stat,

    /// Query list of modified files from external watchman(1) process.
    ///
    /// Watchman uses inotify(2), so this method scales linearly with count of files in
    /// the repository that were actually modified.
    ///
    /// It is user responsibility to get watchman(1) process up and running and preferable
    /// set "WATCHMAN_SOCK" environment variable. One way to set that environment variable
    /// is to put following snippet into ~/.bashrc:
    ///
    /// export WATCHMAN_SOCK=$(watchman get-sockname | jq .sockname -r)
    Watchman,

    /// Try to use watchman(1), and silently fall back on stat should it fail.
    Auto,
}

impl std::str::FromStr for FSScan {
    type Err = String;

    fn from_str(s: &str) -> Result<Self, Self::Err> {
        match s {
            "stat" => Ok(Self::Stat),
            "watchman" => Ok(Self::Watchman),
            "auto" => Ok(Self::Auto),
            _ => Err(format!("Unknown file-system scan method: {s}")),
        }
    }
}

/// ✨  format all your language!
#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
pub struct Cli {
    /// Create a new treefmt.toml.
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

    #[arg(long = "hidden", short = 'H')]
    /// Include hidden files while traversing the tree.
    /// Override with the --no-hidden flag.
    pub hidden: bool,
    /// Overrides the --hidden flag.
    /// Don't include hidden files while traversing the tree.
    #[arg(long, overrides_with = "hidden", hide = true)]
    no_hidden: bool,

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

    /// Log verbosity is based off the number of v used.
    #[clap(flatten)]
    pub verbose: Verbosity<InfoLevel>,

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

    /// Select filesystem scan method (stat, watchman, auto).
    #[arg(long, default_value = "auto")]
    pub fs_scan: FSScan,
}

fn current_dir() -> anyhow::Result<PathBuf> {
    // First we try and read $PWD as it will (hopefully) honour symlinks
    env::var("PWD")
        .map(PathBuf::from)
        // If we can't read the $PWD var then we fall back to use getcwd
        .or_else(|_| {
            warn!("PWD environment variable not set, if current directory is a symlink it will be dereferenced");
            env::current_dir()
        })
        .map_err(anyhow::Error::new)
}

fn parse_path(s: &str) -> anyhow::Result<PathBuf> {
    // Obtain current dir and ensure is absolute
    let cwd = match current_dir() {
        Ok(dir) => dir,
        Err(err) => return Err(anyhow!("{}", err)),
    };
    assert!(cwd.is_absolute());

    // TODO: Include validation for incorrect paths or characters
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
    } else {
        match &cli.config_file {
            None => {
                // Fail if configuration could not be found. This is checked
                // here to avoid aborting before init_cmd.
                return Err(anyhow!(
                    "{} could not be found in {} and up. Use the --init option to create one or specify --config-file if it is in a non-standard location.",
                    config::FILENAME,
                    cli.work_dir.display(),
                ));
            }
            Some(config_file) => {
                if cli.stdin {
                    format_stdin_cmd(
                        &cli.tree_root,
                        &cli.work_dir,
                        config_file,
                        &cli.paths,
                        &cli.formatters,
                    )?
                } else {
                    format_cmd(
                        &cli.tree_root,
                        &cli.work_dir,
                        config_file,
                        &cli.paths,
                        cli.hidden,
                        cli.no_cache,
                        cli.clear_cache,
                        cli.fail_on_change,
                        cli.allow_missing_formatter,
                        &cli.formatters,
                        &cli.fs_scan,
                    )?
                }
            }
        }
    }

    Ok(())
}

#[cfg(test)]
mod tests {
    #[test]
    #[ignore = "std::env::set_var: should not be run in parallel."]
    fn current_dir_prefers_pwd_env_var() {
        use crate::command::current_dir;
        use std::env;
        use std::path::PathBuf;

        let expected_pwd = "/tmp";
        let prev_pwd = env::var("PWD").unwrap();
        env::set_var("PWD", expected_pwd);

        let result = current_dir().unwrap();

        env::set_var("PWD", prev_pwd);

        assert_eq!(result, PathBuf::from(expected_pwd));
    }

    #[test]
    #[ignore = "std::env::set_var: should not be run in parallel."]
    fn current_dir_uses_dereferenced_path_when_pwd_env_var_not_set() {
        use crate::command::current_dir;
        use std::env;

        let expected_pwd = env::current_dir().unwrap();
        let prev_pwd = env::var("PWD").unwrap();
        env::remove_var("PWD");

        let result = current_dir().unwrap();

        env::set_var("PWD", prev_pwd);

        assert_eq!(result, expected_pwd);
    }
}
