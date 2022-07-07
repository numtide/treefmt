use crate::config;
use crate::engine::run_treefmt_stdin;
use crate::expand_path;
use anyhow::anyhow;
use directories::ProjectDirs;
use log::debug;
use std::path::{Path, PathBuf};

pub fn format_stdin_cmd(
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

    // Check that only one path was provided
    if paths.is_empty() {
        return Err(anyhow!(
            "--stdin requires the path of the target file as an argument"
        ));
    } else if paths.len() > 1 {
        return Err(anyhow!(
            "--stdin requires one path but was given {}",
            paths.len()
        ));
    }

    let path = expand_path(paths.first().unwrap(), work_dir);

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
    run_treefmt_stdin(
        &tree_root,
        work_dir,
        &cache_dir,
        &config_file,
        &path,
        selected_formatters,
    )?;

    Ok(())
}
