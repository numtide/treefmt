//! Keep track of evaluations
use crate::{customlog, CmdContext, CLOG};

use anyhow::{anyhow, Error, Result};
use serde::{Deserialize, Serialize};
use sha1::{Digest, Sha1};
use std::collections::BTreeMap;
use std::fs::{read_to_string, File};
use std::io::Write;
use std::path::PathBuf;

#[derive(Debug, Default, Deserialize, Serialize)]
/// RootManifest
pub struct RootManifest {
    /// Map of manifests config based on its formatter
    pub manifest: BTreeMap<String, CmdContext>,
}

/// Create <hex(hash(path-to-treefmt))>.toml and put it in $XDG_CACHE_DIR/treefmt/eval-cache/
pub fn create_manifest(
    treefmt_toml: PathBuf,
    cache_dir: PathBuf,
    cmd_ctx: Vec<CmdContext>,
) -> Result<()> {
    let hash_toml = create_hash(&treefmt_toml)?;

    let mut f = File::create(cache_dir.join(hash_toml))?;
    let map_manifest: BTreeMap<String, CmdContext> = cmd_ctx
        .into_iter()
        .map(|cmd| {
            let name = cmd.name.clone();
            let manifest = CmdContext {
                name: cmd.name,
                path: cmd.path,
                mtime: cmd.mtime,
                work_dir: cmd.work_dir,
                options: cmd.options,
                metadata: cmd.metadata,
            };
            (name, manifest)
        })
        .collect();
    let manifest_toml = RootManifest {
        manifest: map_manifest.clone(),
    };
    f.write_all(
        format!(
            "# {} DO NOT HAND EDIT THIS FILE {}\n\n{}",
            customlog::WARN,
            customlog::WARN,
            toml::to_string_pretty(&manifest_toml)?
        )
        .as_bytes(),
    )?;
    Ok(())
}

/// Read the <hex(hash(path-to-treefmt))>.toml and return list of config's cache evaluation
pub fn read_manifest(treefmt_toml: &PathBuf, cache_dir: &PathBuf) -> Result<RootManifest> {
    let hash_toml = create_hash(&treefmt_toml)?;
    let manifest_toml = cache_dir.join(&hash_toml);

    if manifest_toml.exists() {
        CLOG.debug(&format!("Found {} in: {}", hash_toml, cache_dir.display()));
        let open_file = match read_to_string(&manifest_toml) {
            Ok(file) => file,
            Err(err) => {
                return Err(anyhow!(
                    "Cannot open {} due to {}.",
                    manifest_toml.display(),
                    err
                ))
            }
        };

        let manifest_content: RootManifest = toml::from_str(&open_file)?;
        Ok(manifest_content)
    } else {
        CLOG.debug(&format!("{} not found!", hash_toml));
        Ok(RootManifest {
            manifest: BTreeMap::new(),
        })
    }
}

fn create_hash(treefmt_toml: &PathBuf) -> Result<String> {
    let treefmt_str = match treefmt_toml.to_str() {
        Some(str) => str.as_bytes(),
        None => {
            return Err(anyhow!(
                "{}cannot convert to string slice",
                customlog::ERROR
            ))
        }
    };
    let treefmt_hash = Sha1::digest(treefmt_str);
    let result = hex::encode(treefmt_hash);
    let manifest_toml = format!("{}.toml", result);
    Ok(manifest_toml)
}

/// Checking content of cache's file and current treefmt runs
pub fn check_treefmt(
    treefmt_toml: &PathBuf,
    cmd_context: &[CmdContext],
    cache: &RootManifest,
) -> Result<Vec<CmdContext>> {
    let cache_context = cache.manifest.values();
    let map_ctx: BTreeMap<String, CmdContext> = cmd_context
        .into_iter()
        .map(|cmd| {
            let name = cmd.name.clone();
            let ctx = CmdContext {
                name: cmd.name.clone(),
                mtime: cmd.mtime.clone(),
                path: cmd.path.clone(),
                work_dir: cmd.work_dir.clone(),
                options: cmd.options.clone(),
                metadata: cmd.metadata.clone(),
            };
            (name, ctx)
        })
        .collect();
    let new_cmd_ctx = map_ctx.values();
    let results = new_cmd_ctx.clone().into_iter().zip(cache_context);
    let cache_context: Vec<CmdContext> = results
        .clone()
        .map(|(new, old)| {
            Ok(CmdContext {
                name: new.name.clone(),
                path: new.path.clone(),
                mtime: new.mtime.clone(),
                work_dir: new.work_dir.clone(),
                options: new.options.clone(),
                metadata: if new.path != old.path
                    || new.options != old.options
                    || new.work_dir != old.work_dir
                {
                    // If either the command or the options have changed, invalidate old entries
                    new.metadata.clone()
                } else {
                    new.metadata.difference(&old.metadata).cloned().collect()
                },
            })
        })
        .filter(|c| match c {
            Ok(x) => !x.metadata.is_empty(),
            _ => false,
        })
        .collect::<Result<Vec<CmdContext>, Error>>()?;

    if cache_context.iter().all(|f| f.metadata.is_empty()) {
        CLOG.debug(&format!("No changes found in {}", treefmt_toml.display()));
        return Ok(Vec::new());
    }

    CLOG.info("The following file has changed or newly added:");
    for cmd in &cache_context {
        if !cmd.metadata.is_empty() {
            for p in &cmd.metadata {
                CLOG.info(&format!(
                    " - {} last modification time: {}",
                    p.path.display(),
                    p.mtime
                ));
            }
        }
    }
    // return Err(anyhow!("treefmt failed to run."));
    Ok(cache_context)
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::BTreeMap;

    /// Every same path produce same hash
    #[test]
    fn test_create_hash() -> Result<()> {
        let file_path = PathBuf::from(r"examples/monorepo/treefmt.toml");
        let treefmt_hash = "3076c82c47ced65a86ffe285ac9d941d812bfdad.toml";
        assert_eq!(create_hash(&file_path)?, treefmt_hash);
        Ok(())
    }

    /// Every same path produce same hash
    #[test]
    fn test_check_treefmt() -> Result<()> {
        let treefmt_path = PathBuf::from(r"examples/monorepo/treefmt.toml");
        let cache: RootManifest = RootManifest {
            manifest: BTreeMap::new(),
        };
        let cmd_context: Vec<CmdContext> = Vec::new();

        assert_eq!(
            check_treefmt(&treefmt_path, &cmd_context, &cache)?,
            cmd_context
        );
        Ok(())
    }
}
