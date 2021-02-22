//! The main formatting engine logic should be in this module.

use crate::eval_cache::{check_treefmt, create_manifest, read_manifest, RootManifest};
use crate::{config, CmdContext, CmdMeta, FileMeta, CLOG};
use anyhow::{Error, Result};
use filetime::FileTime;
use rayon::prelude::*;
use std::collections::BTreeSet;
use std::fs::metadata;
use std::iter::Iterator;
use std::path::PathBuf;
use std::process::Command;

/// Run the treefmt
pub fn run_treefmt(cwd: PathBuf, cache_dir: PathBuf, treefmt_toml: PathBuf) -> anyhow::Result<()> {
    let project_config = config::from_path(&treefmt_toml)?;

    // Once the treefmt found the $XDG_CACHE_DIR/treefmt/eval-cache/ folder,
    // it will try to scan the manifest and passed it into check_treefmt function
    let old_ctx = create_command_context(&cwd, &project_config)?;
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

    println!("===========================");
    for c in context {
        if !c.metadata.is_empty() {
            println!("{}", c.command);
            println!(
                "Working Directory: {}",
                c.work_dir.display()
            );
            println!("Files:");
            for m in &c.metadata {
                let path = &m.path;
                println!(" - {}", path.display());
            }
            println!("===========================");
        }
    }
    // TODO: report errors (both Err(_), and Ok(bad status))
    let _outputs: Vec<std::io::Result<std::process::Output>> = context
        .par_iter()
        .map(|c| {
            let arg = &c.options;
            let mut cmd_arg = Command::new(&c.path);
            // Set the command to run under its working directory.
            cmd_arg.current_dir(&c.work_dir);
            // Append the default options to the command.
            cmd_arg.args(arg);
            // Append all of the file paths to format.
            let paths = c.metadata.iter().map(|f| &f.path);
            cmd_arg.args(paths);
            // And run
            cmd_arg.output()
        })
        .collect();

    if mfst.manifest.is_empty() && ctxs.is_empty() {
        CLOG.info("First time running treefmt");
        CLOG.info("capturing formatted file's state...");
        create_manifest(treefmt_toml, cache_dir, old_ctx)?;
    } else {
        // Read the current status of files and insert into the manifest.
        let new_mfst: Vec<CmdContext> = mfst
            .manifest
            .values()
            .map(|e| e.clone().update_meta())
            .collect::<Result<Vec<CmdContext>, Error>>()?;

        println!("Format successful");
        println!("capturing formatted file's state...");
        create_manifest(treefmt_toml, cache_dir, new_mfst)?;
    }

    Ok(())
}

/// Convert glob pattern into list of pathBuf
pub fn glob_to_path(
    cwd: &PathBuf,
    includes: &[String],
    excludes: &[String],
) -> anyhow::Result<Vec<PathBuf>> {
    use ignore::{overrides::OverrideBuilder, WalkBuilder};

    let mut overrides_builder = OverrideBuilder::new(cwd);

    for include in includes {
        overrides_builder.add(include)?;
    }

    for exclude in excludes {
        overrides_builder.add(&format!("!{}", exclude))?;
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
            CLOG.warn("Duplicated file detected:");
            CLOG.warn(&format!(" - {} ", p.display()));
            CLOG.warn("Maybe you want to format one file with different formatter?");
        }
    }
    Ok(filemeta)
}

/// Creating command configuration based on treefmt.toml
pub fn create_command_context(
    cwd: &PathBuf,
    toml_content: &config::Root,
) -> Result<Vec<CmdContext>> {
    let cmd_context: Vec<CmdContext> = toml_content
        .formatter
        .values()
        .map(|config| {
            let list_files = glob_to_path(&cwd.to_path_buf(), &config.includes, &config.excludes)?;
            let cmd_meta = CmdMeta::new(config.command.clone())?;
            Ok(CmdContext {
                command: cmd_meta.name,
                mtime: cmd_meta.mtime,
                path: cmd_meta.path,
                options: config.options.clone(),
                work_dir: config.work_dir.clone(),
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
        let includes = vec!["*.rs".to_string()];
        let excludes: Vec<String> = vec![];
        let glob_path = PathBuf::from(r"examples/rust/src/main.rs");
        let mut vec_path = Vec::new();
        vec_path.push(glob_path);
        assert_eq!(glob_to_path(&cwd, &includes, &excludes)?, vec_path);
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
