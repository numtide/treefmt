use crate::command::FSScan;
use crate::engine::run_treefmt;
use anyhow::anyhow;
use directories::ProjectDirs;
use log::{debug, warn};
use std::path::{Path, PathBuf};
use tokio;
use watchman_client::prelude::Connector;

#[tokio::main]
pub async fn format_cmd(
    tree_root: &Option<PathBuf>,
    work_dir: &Path,
    config_file: &Path,
    paths: &[PathBuf],
    hidden: bool,
    no_cache: bool,
    clear_cache: bool,
    fail_on_change: bool,
    allow_missing_formatter: bool,
    selected_formatters: &Option<Vec<String>>,
    fs_scan: &FSScan,
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

    let client = match fs_scan {
        FSScan::Stat => None,
        FSScan::Watchman => {
            // This is important. Subprocess wastes ~20ms.
            if !std::env::var("WATCHMAN_SOCK").is_ok() {
                warn!(
                    "Environment variable `WATCHMAN_SOCK' is not set, falling back on subprocess"
                );
            };
            match Connector::new().connect().await {
                Err(e) => return Err(anyhow!("watchman is not available (err = {:?})", e)),
                Ok(c) => Some(c),
            }
        }
        FSScan::Auto => Connector::new().connect().await.ok(),
    };

    // Finally run the main formatter logic from the engine.
    run_treefmt(
        &tree_root,
        work_dir,
        &cache_dir,
        config_file,
        &paths,
        hidden,
        no_cache,
        clear_cache,
        fail_on_change,
        allow_missing_formatter,
        selected_formatters,
        &client.as_ref(),
    )
    .await?;

    Ok(())
}
