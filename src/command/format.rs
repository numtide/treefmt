use crate::engine::run_treefmt;
use anyhow::anyhow;
use directories::ProjectDirs;
use log::debug;
use std::fs;
use std::path::{Path, PathBuf};

pub fn format_cmd(
    tree_root: &Option<PathBuf>,
    work_dir: &Path,
    config_file: &Path,
    paths: &[PathBuf],
    no_cache: bool,
    clear_cache: bool,
    fail_on_change: bool,
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
    // Make sure the cache directory exists.
    fs::create_dir_all(&cache_dir)?;

    debug!(
        "tree_root={} work_dir={} cache_dir={} config_file={} paths={:?}",
        tree_root.display(),
        work_dir.display(),
        cache_dir.display(),
        config_file.display(),
        paths
    );

    // Finally run the main formatter logic from the engine.
    run_treefmt(
        &tree_root,
        work_dir,
        &cache_dir,
        config_file,
        &paths,
        no_cache,
        clear_cache,
        fail_on_change,
    )?;

    Ok(())
}
