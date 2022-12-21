use anyhow::{anyhow, Context};
use clap::Parser;
use clap_verbosity_flag::Verbosity;
use directories::ProjectDirs;
use log::{debug, error, info};
use std::{
    env, fs, include_bytes,
    path::{Path, PathBuf},
    process::ExitCode,
};

use treefmt::{config, engine, expand_path};

/// Command line options for treefmt.
#[derive(Parser, Debug)]
#[command(author, version, about, long_about = None)]
struct Cli {
    /// Create a new treefmt.toml
    #[arg(short, long, default_value_t = false)]
    init: bool,

    /// Format the content passed in stdin.
    #[arg(long, default_value_t = false, conflicts_with("init"))]
    stdin: bool,

    /// Ignore the evaluation cache entirely. Useful for CI.
    #[arg(long, conflicts_with("stdin"), conflicts_with("init"))]
    no_cache: bool,

    /// Reset the evaluation cache. Use in case the cache is not precise enough.
    #[arg(short, long, default_value_t = false)]
    clear_cache: bool,

    /// Exit with error if any changes were made. Useful for CI.
    #[arg(
        long,
        default_value_t = false,
        conflicts_with("stdin"),
        conflicts_with("init")
    )]
    fail_on_change: bool,

    /// Do not exit with error if a configured formatter is missing.
    #[arg(long, default_value_t = false)]
    allow_missing_formatter: bool,

    #[clap(flatten)]
    verbose: Verbosity,

    /// Run as if treefmt was started in <work-dir> instead of the current working directory.
    #[arg(short, default_value = ".", value_parser = parse_path)]
    work_dir: PathBuf,

    /// Set the path to the tree root directory. Defaults to the folder holding the treefmt.toml file.
    #[arg(long, env = "PRJ_ROOT", default_value = ".", value_parser = parse_path)]
    tree_root: Option<PathBuf>,

    /// Run with the specified config file, which is not required to be in the tree to be formatted.
    #[arg(long, value_parser = parse_path)]
    config_file: Option<PathBuf>,

    /// Paths to format. Defaults to formatting the whole tree.
    #[arg()]
    paths: Vec<PathBuf>,

    /// Select formatters name to apply. Defaults to all formatters.
    #[arg(short, long)]
    formatters: Option<Vec<String>>,
}

fn main() -> ExitCode {
    let cli = Cli::parse();

    // Configure the logger
    env_logger::builder()
        .format_timestamp(None)
        .format_target(false)
        .filter_level(cli.verbose.log_level_filter())
        .init();

    // Run the app!
    match run_arg_command(cli) {
        Ok(()) => ExitCode::SUCCESS,
        Err(e) => {
            error!("{}", e);
            ExitCode::FAILURE
        }
    }
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
    Ok(expand_path(&path, &cwd))
}

fn run_arg_command(mut cli: Cli) -> anyhow::Result<()> {
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

    if cli.init {
        run_init(&cli.work_dir)?
    } else if cli.stdin {
        run_format_stdin(&cli.tree_root, &cli.work_dir, &cli.paths, &cli.formatters)?
    } else {
        // Fail if configuration could not be found. This is checked
        // here to avoid aborting before init_cmd.
        if cli.config_file.is_none() {
            return Err(anyhow!(
                "{} could not be found in {} and up. Use the --init option to create one or specify --config-file if it is in a non-standard location.",
                treefmt::config::FILENAME,
                cli.work_dir.display(),
            ));
        }

        run_format(
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

/// Creates a new treefmt.toml file as a template
fn run_init(work_dir: &Path) -> anyhow::Result<()> {
    let file_path = work_dir.join(config::FILENAME);
    // TODO: detect if file exists
    fs::write(&file_path, include_bytes!("init_treefmt.toml")).with_context(|| {
        format!(
            "{} `{}`",
            console::style("Error writing").bold().red(),
            console::style(file_path.display()).bold()
        )
    })?;

    info!("Generated treefmt template at {}", file_path.display());
    Ok(())
}

/// Performs the formatting of the tree
fn run_format(
    tree_root: &Option<PathBuf>,
    work_dir: &Path,
    config_file: &Path,
    paths: &[PathBuf],
    no_cache: bool,
    clear_cache: bool,
    fail_on_change: bool,
    allow_missing_formatter: bool,
    selected_formatters: &Option<Vec<String>>,
) -> anyhow::Result<()> {
    let proj_dirs = match ProjectDirs::from("com", "NumTide", "treefmt") {
        Some(x) => x,
        None => {
            return Err(anyhow!(
                "Could not find the project directories. On Unix, check if the HOME env is missing."
            ))
        }
    };

    // Default the tree root to the folder that contains the config file
    let tree_root = tree_root.clone().unwrap_or_else(|| {
        // unwrap: since the config_file is a file, there MUST be a parent folder.
        config_file.parent().unwrap().to_path_buf()
    });

    // Default to the tree root if no paths have been given
    let paths = if paths.is_empty() {
        vec![tree_root.clone()]
    } else {
        paths.to_owned()
    };

    let cache_dir = proj_dirs.cache_dir().join("eval-cache");

    debug!(
        "tree_root={} work_dir={} cache_dir={} config_file={} paths={:?}",
        tree_root.display(),
        work_dir.display(),
        cache_dir.display(),
        config_file.display(),
        paths
    );

    // Finally run the main formatter logic from the engine.
    engine::run_treefmt(
        &tree_root,
        work_dir,
        &cache_dir,
        config_file,
        &paths,
        no_cache,
        clear_cache,
        fail_on_change,
        allow_missing_formatter,
        selected_formatters,
    )?;

    Ok(())
}

/// Performs the formatting of the stdin
fn run_format_stdin(
    tree_root: &Option<PathBuf>,
    work_dir: &Path,
    paths: &[PathBuf],
    selected_formatters: &Option<Vec<String>>,
) -> anyhow::Result<()> {
    let proj_dirs = match ProjectDirs::from("com", "NumTide", "treefmt") {
        Some(x) => x,
        None => {
            return Err(anyhow!(
                "Could not find the project directories. On Unix, check if the HOME env is missing."
            ))
        }
    };

    // Search for the treefmt.toml from there.
    let config_file = match config::lookup(work_dir) {
        Some(path) => path,
        None => {
            return Err(anyhow!(
                "{} could not be found in {} and up. Use the --init option to create one.",
                config::FILENAME,
                work_dir.display()
            ));
        }
    };

    // Default the tree root to the folder that contains the config file
    let tree_root = tree_root.clone().unwrap_or_else(|| {
        // unwrap: since the config_file is a file, there MUST be a parent folder.
        config_file.clone().parent().unwrap().to_path_buf()
    });

    let path = treefmt::expand_path(paths.first().unwrap(), work_dir);

    let cache_dir = proj_dirs.cache_dir().join("eval-cache");

    debug!(
        "tree_root={} work_dir={} cache_dir={} config_file={} path={}",
        tree_root.display(),
        work_dir.display(),
        cache_dir.display(),
        config_file.display(),
        path.display()
    );

    // Finally run the main formatter logic from the engine.
    engine::run_treefmt_stdin(
        &tree_root,
        work_dir,
        &cache_dir,
        &config_file,
        &path,
        selected_formatters,
    )?;

    Ok(())
}
