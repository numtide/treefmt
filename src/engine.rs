//! The main formatting engine logic should be in this module.

use crate::formatters::{
    check::check_treefmt,
    manifest::{create_manifest, read_manifest},
    RootManifest,
};
use crate::{customlog, CmdContext, FileExtensions, FileMeta, Root, CLOG};
use anyhow::{anyhow, Error, Result};
use filetime::FileTime;
use rayon::prelude::*;
use std::collections::BTreeSet;
use std::fs::{metadata, read_to_string};
use std::iter::{IntoIterator, Iterator};
use std::path::PathBuf;
use which::which;
use xshell::cmd;

/// Make sure that formatter binary exists. This also for other formatter
pub fn check_bin(command: &str) -> Result<()> {
    let cmd_bin = command.split_ascii_whitespace().next().unwrap_or("");
    if let Ok(path) = which(cmd_bin) {
        CLOG.info(&format!("Found {} at {}", cmd_bin, path.display()));
        return Ok(());
    }
    anyhow::bail!(
        "Failed to locate formatter named {}. \
        Please make sure it is available in your PATH",
        command
    )
}

/// Run the treefmt
pub fn run_treefmt(cwd: PathBuf, cache_dir: PathBuf) -> anyhow::Result<()> {
    let treefmt_toml = cwd.join("treefmt.toml");

    // Once the treefmt found the $XDG_CACHE_DIR/treefmt/eval-cache/ folder,
    // it will try to scan the manifest and passed it into check_treefmt function
    let old_ctx = create_command_context(&treefmt_toml)?;
    // TODO: Resolve all of the formatters paths. If missing, print an error, remove the formatters from the list and continue.
    // Load the manifest if it exists, otherwise start with empty manifest
    let mfst: RootManifest = read_manifest(&treefmt_toml, &cache_dir)?;
    // Compare the list of files with the manifest, keep the ones that are not in the manifest
    let ctxs = check_treefmt(&treefmt_toml, &old_ctx, &mfst)?;
    let context = if mfst.manifest.is_empty() && ctxs.is_empty() {
        &old_ctx
    } else {
        &ctxs
    };

    if !treefmt_toml.exists() {
        return Err(anyhow!(
            "{}treefmt.toml not found, please run --init command",
            customlog::ERROR
        ));
    }

    for c in context {
        check_bin(&c.command)?;
    }

    println!("===========================");
    for c in context {
        if !c.metadata.is_empty() {
            println!("Command: {}", c.command);
            println!("Files:");
            for m in &c.metadata {
                let path = &m.path;
                println!(" - {}", path.display());
            }
            println!("===========================");
        }
    }

    // TODO: report errors (both Err(_), and Ok(bad status))
    let _outputs: Vec<xshell::Result<std::process::Output>> = context
        .par_iter()
        .map(|c| {
            let arg = &c.options;
            let cmd_arg = &c.command;
            let paths = c.metadata.iter().map(|f| &f.path);
            cmd!("{cmd_arg} {arg...} {paths...}").output()
        }).collect();

    if mfst.manifest.is_empty() || ctxs.is_empty() {
        create_manifest(treefmt_toml, cache_dir, old_ctx)?;
    } else {
        // Read the current status of files and insert into the manifest.
        let new_ctx = create_command_context(&treefmt_toml)?;
        println!("Format successful");
        println!("capturing formatted file's state...");
        create_manifest(treefmt_toml, cache_dir, new_ctx)?;
    }

    Ok(())
}

/// Convert glob pattern into list of pathBuf
pub fn glob_to_path (
    cwd: &PathBuf,
    extensions: &FileExtensions,
    includes: &Option<Vec<String>>,
    excludes: &Option<Vec<String>>,
) -> anyhow::Result<Vec<PathBuf>> {
    use ignore::{overrides::OverrideBuilder, WalkBuilder};

    let mut overrides_builder = OverrideBuilder::new(cwd);

    if let Some(includes) = includes {
        for include in includes {
            // Remove trailing `/` as we add one explicitly in the override
            let include = include.trim_end_matches('/');
            for extension in extensions.into_iter() {
                overrides_builder.add(&format!("{}/**/{}", include, extension))?;
            }
        }
    } else {
        for extension in extensions.into_iter() {
            overrides_builder.add(&extension)?;
        }
    }

    if let Some(excludes) = excludes {
        for exclude in excludes {
            overrides_builder.add(&format!("!{}", exclude))?;
        }
    }

    let overrides = overrides_builder.build()?;

    Ok(WalkBuilder::new(cwd)
        .overrides(overrides)
        .build()
        .filter_map(|e| {
            e.ok().and_then(|f| {
                match f.file_type() {
                    // Skip directory entries
                    Some(t) if t.is_dir() => None,
                    _ => Some(f.into_path()),
                }
            })
        })
        .collect())
}

/// Convert each PathBuf into FileMeta
/// FileMeta consist of file's path and its modification times
pub fn path_to_filemeta(paths: Vec<PathBuf>) -> Result<BTreeSet<FileMeta>> {
    let mut filemeta = BTreeSet::new();
    for p in paths {
        let metadata = metadata(&p)?;
        let mtime = FileTime::from_last_modification_time(&metadata).unix_seconds();
        if !filemeta.insert(FileMeta {
            mtimes: mtime,
            path: p.clone(),
        }) {
            CLOG.warn(&format!("Duplicated file detected:"));
            CLOG.warn(&format!(" - {:?} ", p.display()));
            CLOG.warn(&format!(
                "Maybe you want to format one file with different formatter?"
            ));
        }
    }
    Ok(filemeta)
}

/// Creating command configuration based on treefmt.toml
pub fn create_command_context(treefmt_toml: &PathBuf) -> Result<Vec<CmdContext>> {
    let open_treefmt = match read_to_string(treefmt_toml) {
        Ok(file) => file,
        Err(err) => {
            return Err(anyhow!(
                "cannot open {} due to {}.",
                treefmt_toml.display(),
                err
            ))
        }
    };

    let cwd = match treefmt_toml.parent() {
        Some(path) => path,
        None => {
            return Err(anyhow!(
                "{}treefmt.toml not found, please run --init command",
                customlog::ERROR
            ))
        }
    };

    let toml_content: Root = toml::from_str(&open_treefmt)?;
    let cmd_context: Vec<CmdContext> = toml_content
        .formatters
        .values()
        .map(|config| {
            let list_files = glob_to_path(
                &cwd.to_path_buf(),
                &config.files,
                &config.includes,
                &config.excludes,
            )?;
            Ok(CmdContext {
                command: config.command.clone().unwrap_or_default(),
                options: config.options.clone().unwrap_or_default(),
                metadata: path_to_filemeta(list_files)?,
            })
        })
        .collect::<Result<Vec<CmdContext>, Error>>()?;
    Ok(cmd_context)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeSet;

    /// Transforming glob into file path
    #[test]
    fn test_glob_to_path() -> Result<()> {
        let cwd = PathBuf::from(r"examples");
        let file_ext = FileExtensions::SingleFile("*.rs".to_string());
        let glob_path = PathBuf::from(r"examples/rust/src/main.rs");
        let mut vec_path = Vec::new();
        vec_path.push(glob_path);
        assert_eq!(glob_to_path(&cwd, &file_ext, &None, &None)?, vec_path);
        Ok(())
    }

    /// Transforming path into FileMeta
    #[test]
    fn test_path_to_filemeta() -> Result<()> {
        let file_path = PathBuf::from(r"examples/rust/src/main.rs");
        let metadata = metadata(&file_path)?;
        let mtime = FileTime::from_last_modification_time(&metadata).unix_seconds();
        let mut vec_path = Vec::new();
        vec_path.push(file_path);
        let file_meta = FileMeta {
            mtimes: mtime,
            path: PathBuf::from(r"examples/rust/src/main.rs"),
        };
        let mut set_filemeta = BTreeSet::new();
        set_filemeta.insert(file_meta);
        assert_eq!(path_to_filemeta(vec_path)?, set_filemeta);
        Ok(())
    }
}
