use crate::{emoji, CLOG};

use super::tool::CmdContext;
use anyhow::{anyhow, Result};
use hex;
use serde::{Deserialize, Serialize};
use sha1::{Digest, Sha1};
use std::collections::BTreeMap;
use std::fs::{read_to_string, File};
use std::io::Write;
use std::path::PathBuf;
use std::str;

/// Create <hex(hash(path-to-prjfmt))>.toml and put it in $XDG_CACHE_DIR/prjfmt/eval-cache/
pub fn create_prjfmt_manifest(
    prjfmt_toml: PathBuf,
    cache_dir: PathBuf,
    cmd_ctx: Vec<CmdContext>,
) -> Result<()> {
    let hash_toml = create_prjfmt_hash(&prjfmt_toml)?;

    let mut f = File::create(cache_dir.as_path().join(hash_toml))?;
    let map_manifest: BTreeMap<String, CmdContext> = cmd_ctx
        .into_iter()
        .map(|cmd| {
            let prjfmt = cmd.command;
            let manifest = CmdContext {
                command: prjfmt.to_string(),
                options: cmd.options,
                metadata: cmd.metadata,
            };
            (prjfmt.to_string(), manifest)
        })
        .collect();
    let manifest_toml = RootManifest {
        manifest: map_manifest,
    };
    f.write_all(
        format!(
            "# {} DO NOT HAND EDIT THIS FILE {}\n\n{}",
            emoji::WARN,
            emoji::WARN,
            toml::to_string_pretty(&manifest_toml)?
        )
        .as_bytes(),
    )?;
    Ok(())
}

/// Read the <hex(hash(path-to-prjfmt))>.toml and return list of config's cache evaluation
pub fn read_prjfmt_manifest(prjfmt_toml: &PathBuf, path: &PathBuf) -> Result<RootManifest> {
    let hash_toml = create_prjfmt_hash(&prjfmt_toml)?;
    let manifest_toml = path.as_path().join(&hash_toml);

    if manifest_toml.as_path().exists() {
        CLOG.info(&format!("Found {} in: {}", hash_toml, path.display()));
        let open_file = match read_to_string(manifest_toml.as_path()) {
            Ok(file) => file,
            Err(err) => {
                return Err(anyhow!(
                    "cannot open {} due to {}.",
                    manifest_toml.display(),
                    err
                ))
            }
        };

        let manifest_content: RootManifest = toml::from_str(&open_file)?;
        Ok(manifest_content)
    } else {
        CLOG.warn(&format!("{} not found!", hash_toml));
        Ok(RootManifest {
            manifest: BTreeMap::new(),
        })
    }
}

fn create_prjfmt_hash(prjfmt_toml: &PathBuf) -> Result<String> {
    let prjfmt_str = match prjfmt_toml.to_str() {
        Some(str) => str.as_bytes(),
        None => return Err(anyhow!("{}cannot convert to string slice", emoji::ERROR)),
    };
    let prjfmt_hash = Sha1::digest(prjfmt_str);
    let result = hex::encode(prjfmt_hash);
    let manifest_toml = format!("{}.toml", result);
    Ok(manifest_toml)
}

#[derive(Debug, Deserialize, Serialize)]
/// RootManifest
pub struct RootManifest {
    /// Map of manifests config based on its formatter
    pub manifest: BTreeMap<String, CmdContext>,
}
