use crate::config;
use crate::engine::run_treefmt;
use crate::CLOG;
use anyhow::anyhow;
use directories::ProjectDirs;
use std::fs;
use std::path::PathBuf;

pub fn format_cmd(work_dir: &PathBuf, paths: &[PathBuf]) -> anyhow::Result<()> {
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

    let cache_dir = proj_dirs.cache_dir().join("eval-cache");
    // Make sure the cache directory exists.
    fs::create_dir_all(&cache_dir)?;

    CLOG.debug(&format!(
        "work_dir={} cache_dir={} config_file={} paths={:?}",
        work_dir.display(),
        cache_dir.display(),
        config_file.display(),
        paths
    ));

    // Finally run the main formatter logic from the engine.
    run_treefmt(&work_dir, &cache_dir, &config_file, &paths)?;

    Ok(())
}
