use super::lookup_prjfmt_toml;
use crate::engine::run_prjfmt;
use crate::CLOG;
use anyhow::anyhow;
use std::{env, path::PathBuf};
use std::fs;
use std::path::Path;

pub fn format_cmd(path: Option<PathBuf>) -> anyhow::Result<()> {
    let cwd = env::current_dir()?;
    let cfg_dir = match path {
        Some(p) => p,
        None => lookup_prjfmt_toml(cwd)?,
    };

    let prjfmt_toml = cfg_dir.join("prjfmt.toml");
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

    if prjfmt_toml.exists() {
        CLOG.debug(&format!(
            "Found {} at {}",
            prjfmt_toml.display(),
            cfg_dir.display()
        ));
        CLOG.debug(&format!("Change current directory into: {}", cfg_dir.display()));
        let cache_dir = Path::new(&xdg_cache_dir).join("prjfmt/eval-cache");
        fs::create_dir_all(&cache_dir)?;
        run_prjfmt(cfg_dir, cache_dir)?;
    } else {
        CLOG.error(
            "file prjfmt.toml couldn't be found. Run `--init` to generate the default setting",
        );
    }
    Ok(())
}
