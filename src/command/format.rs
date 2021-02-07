use crate::command::Cli;
use crate::formatters::tool::run_prjfmt;
use crate::CLOG;
use anyhow::anyhow;
use std::env;
use std::fs;
use std::path::Path;

pub fn format_cmd(cli: Cli) -> anyhow::Result<()> {
    let cwd = match cli.files {
        Some(path) => path,
        None => env::current_dir()?,
    };

    let prjfmt_toml = cwd.as_path().join("prjfmt.toml");
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
                    home_cache.as_path().display().to_string()
                }
                Err(err) => return Err(anyhow!("cannot open HOME due to {}.", err)),
            }
        }
    };

    if prjfmt_toml.as_path().exists() {
        CLOG.debug(&format!(
            "Found {} at {}",
            prjfmt_toml.display(),
            cwd.display()
        ));
        CLOG.debug(&format!("Change current directory into: {}", cwd.display()));
        let cache_dir = Path::new(&xdg_cache_dir).join("prjfmt/eval-cache");
        fs::create_dir_all(&cache_dir)?;
        run_prjfmt(cwd, cache_dir)?;
    } else {
        CLOG.error(
            "file prjfmt.toml couldn't be found. Run `--init` to generate the default setting",
        );
    }
    Ok(())
}
