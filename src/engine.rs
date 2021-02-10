//! The main formatting engine logic should be in this module.

use crate::formatters::{
    check::check_prjfmt,
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

/// Run the prjfmt
//
// 1. Find and load prjfmt.toml
// 1b. Resolve all of the formatters paths. If missing, print an error, remove the formatters from the list and continue.
// 2. Load the manifest if it exists, otherwise start with empty manifest
// 3. Get the list of files, use the ones passed as argument if not empty, other default to all.
//     Errorr if a file belongs to two formatters
//    => HashMap<formatter>(files, mtimes) // map A
// 4. Compare the list of files with the manifest, keep the ones that are not in the manifest. // map B
// 5. Iterate over each formatter (in parallel)
//      a. Run the formatter with the list of files
//      b. Collect the new list of (files, mtimes) and return that // map C
// 6. Merge map C into map B. Write this as the new manifest.

pub fn run_prjfmt(cwd: PathBuf, cache_dir: PathBuf) -> anyhow::Result<()> {
    let prjfmt_toml = cwd.as_path().join("prjfmt.toml");

    // Once the prjfmt found the $XDG_CACHE_DIR/prjfmt/eval-cache/ folder,
    // it will try to scan the manifest and passed it into check_prjfmt function
    let manifest: RootManifest = read_manifest(&prjfmt_toml, &cache_dir)?;
    let old_ctx = create_command_context(&prjfmt_toml)?;
    let ctxs = check_prjfmt(&prjfmt_toml, &old_ctx, &manifest)?;

    let context = if manifest.manifest.is_empty() && ctxs.is_empty() {
        &old_ctx
    } else {
        &ctxs
    };

    if !prjfmt_toml.as_path().exists() {
        return Err(anyhow!(
            "{}prjfmt.toml not found, please run --init command",
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
            // TODO: go over all the paths, and collect the ones that have a new mtime.
            // => list (file, mtime)
        })
        .collect();

    let new_ctx: Vec<CmdContext> = old_ctx
        .iter()
        .flat_map(|octx| {
            ctxs.iter().clone().map(move |c| {
                if c.command == octx.command {
                    CmdContext {
                        command: c.command.clone(),
                        options: c.options.clone(),
                        metadata: octx.metadata.union(&c.metadata).cloned().collect(),
                    }
                } else {
                    octx.clone()
                }
            })
        })
        .collect();

    if manifest.manifest.is_empty() || ctxs.is_empty() {
        create_manifest(prjfmt_toml, cache_dir, old_ctx)?;
    } else {
        println!("Format successful");
        println!("capturing formatted file's state...");
        create_manifest(prjfmt_toml, cache_dir, new_ctx)?;
    }

    Ok(())
}

/// Convert glob pattern into list of pathBuf
pub fn glob_to_path(
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
            e.ok().and_then(|e| {
                match e.file_type() {
                    // Skip directory entries
                    Some(t) if t.is_dir() => None,
                    _ => Some(e.into_path()),
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
            // return Err(anyhow!("prjfmt failed to run."));
        }
    }
    Ok(filemeta)
}

/// Creating command configuration based on prjfmt.toml
pub fn create_command_context(prjfmt_toml: &PathBuf) -> Result<Vec<CmdContext>> {
    let open_prjfmt = match read_to_string(prjfmt_toml.as_path()) {
        Ok(file) => file,
        Err(err) => {
            return Err(anyhow!(
                "cannot open {} due to {}.",
                prjfmt_toml.display(),
                err
            ))
        }
    };

    let cwd = match prjfmt_toml.parent() {
        Some(path) => path,
        None => {
            return Err(anyhow!(
                "{}prjfmt.toml not found, please run --init command",
                customlog::ERROR
            ))
        }
    };

    let toml_content: Root = toml::from_str(&open_prjfmt)?;
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
        let cwd = PathBuf::from(r"examples/monorepo");
        let file_ext = FileExtensions::SingleFile("*.rs".to_string());
        let glob_path = PathBuf::from(r"examples/monorepo/rust/src/main.rs");
        let mut vec_path = Vec::new();
        vec_path.push(glob_path);
        assert_eq!(glob_to_path(&cwd, &file_ext, &None, &None)?, vec_path);
        Ok(())
    }

    /// Transforming path into FileMeta
    #[test]
    fn test_path_to_filemeta() -> Result<()> {
        let file_path = PathBuf::from(r"examples/monorepo/rust/src/main.rs");
        let metadata = metadata(&file_path)?;
        let mtime = FileTime::from_last_modification_time(&metadata).unix_seconds();
        let mut vec_path = Vec::new();
        vec_path.push(file_path);
        let file_meta = FileMeta {
            mtimes: mtime,
            path: PathBuf::from(r"examples/monorepo/rust/src/main.rs"),
        };
        let mut set_filemeta = BTreeSet::new();
        set_filemeta.insert(file_meta);
        assert_eq!(path_to_filemeta(vec_path)?, set_filemeta);
        Ok(())
    }
}
