use crate::config;
use crate::engine::run_treefmt;
use crate::CLOG;
use anyhow::anyhow;
use directories::ProjectDirs;
use std::fs;
use std::path::{Path, PathBuf};

pub fn format_cmd(
    tree_root: &Option<PathBuf>,
    work_dir: &Path,
    paths: &[PathBuf],
    clear_cache: bool,
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
    let config_file = match config::lookup(&work_dir) {
        Some(path) => path,
        None => {
            return Err(anyhow!(
                "{} could not be found in {} and up. Use the --init option to create one.",
                config::FILENAME,
                work_dir.display()
            ))
        }
    };

    // Default the tree root to the folder that contains the config file
    let tree_root = tree_root.clone().unwrap_or_else(|| {
        // unwrap: since the config_file is a file, there MUST be a parent folder.
        config_file.clone().parent().unwrap().to_path_buf()
    });

    let cache_dir = proj_dirs.cache_dir().join("eval-cache");
    // Make sure the cache directory exists.
    fs::create_dir_all(&cache_dir)?;

    CLOG.debug(&format!(
        "tree_root={} work_dir={} cache_dir={} config_file={} paths={:?}",
        tree_root.display(),
        work_dir.display(),
        cache_dir.display(),
        config_file.display(),
        paths
    ));

    // Finally run the main formatter logic from the engine.
    run_treefmt(
        &tree_root,
        &work_dir,
        &cache_dir,
        &config_file,
        &paths,
        clear_cache,
    )?;

    Ok(())
}
