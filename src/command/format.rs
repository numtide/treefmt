use crate::config;
use crate::engine::run_treefmt;
use crate::CLOG;
use anyhow::anyhow;
use std::fs;
use std::path::Path;
use std::{env, path::PathBuf};

pub fn format_cmd(work_dir: PathBuf) -> anyhow::Result<()> {
    // Search for the treefmt.toml from there.
    let treefmt_toml = match config::lookup(&work_dir) {
        Some(p) => p,
        None => {
            return Err(anyhow!(
                "{} could not be found in {} and up. Use the --init option to create one.",
                config::FILENAME,
                work_dir.display()
            ))
        }
    };
    // We assume that the parent always exists since the file is contained in a folder.
    let treefmt_root = treefmt_toml.parent().unwrap();

    CLOG.debug(&format!(
        "Found {} at {}",
        treefmt_toml.display(),
        treefmt_root.display()
    ));

    // Find the location of the cache directory to store the eval-cache manifests.
    let xdg_cache_dir = match env::var("XDG_CACHE_DIR") {
        Ok(path) => path,
        Err(err) => {
            CLOG.debug(&format!("{}", err));
            match env::var("HOME") {
                Ok(h) => {
                    let home_cache = Path::new(&h).join(".cache");
                    CLOG.debug(&format!(
                        "Set the $XDG_CACHE_DIR to {}",
                        home_cache.display()
                    ));
                    home_cache.display().to_string()
                }
                Err(err) => return Err(anyhow!("cannot open HOME due to {}.", err)),
            }
        }
    };
    let cache_dir = Path::new(&xdg_cache_dir).join("treefmt/eval-cache");
    // Make sure the cache directory exists.
    fs::create_dir_all(&cache_dir)?;

    // Finally run the main formatter logic from the engine.
    run_treefmt(work_dir.to_path_buf(), cache_dir, treefmt_toml)?;

    Ok(())
}
