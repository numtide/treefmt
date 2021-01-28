use super::manifest::create_prjfmt_manifest;
use crate::formatters::check::check_prjfmt;
use crate::formatters::manifest::{read_prjfmt_manifest, RootManifest};
use crate::{emoji, CLOG};
use anyhow::{anyhow, Error, Result};
use filetime::FileTime;
use rayon::prelude::*;
use serde::{Deserialize, Serialize};
use std::cmp::Ordering;
use std::collections::{BTreeMap, BTreeSet};
use std::fs::{metadata, read_to_string};
use std::iter::{IntoIterator, Iterator};
use std::path::PathBuf;
use xshell::cmd;

/// Make sure that formatter binary exists. This also for other formatter
pub fn check_bin(command: &str) -> Result<()> {
    let cmd_bin = command.split_ascii_whitespace().next().unwrap_or("");
    if let Ok(str) = cmd!("which {cmd_bin}").read() {
        CLOG.info(&format!("Found {} at {}", cmd_bin, str));
        return Ok(());
    }
    anyhow::bail!(
        "Failed to locate formatter named {}. \
        Please make sure it is available in your PATH",
        command
    )
}

/// Run the prjfmt
pub fn run_prjfmt(cwd: PathBuf, cache_dir: PathBuf) -> anyhow::Result<()> {
    let prjfmt_toml = cwd.as_path().join("prjfmt.toml");

    // Once the prjfmt found the $XDG_CACHE_DIR/prjfmt/eval-cache/ folder,
    // it will try to scan the manifest and passed it into check_prjfmt function
    let manifest: RootManifest = read_prjfmt_manifest(&prjfmt_toml, &cache_dir)?;
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
            emoji::ERROR
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
        .flat_map(|c| {
            c.metadata.par_iter().cloned().map(move |m| {
                let arg = &c.args;
                let cmd_arg = &c.command;
                let path = &m.path;
                cmd!("{cmd_arg} {arg...} {path}").output()
            })
        })
        .collect();

    let new_ctx: Vec<CmdContext> = old_ctx
        .iter()
        .flat_map(|octx| {
            ctxs.iter().clone().map(move |c| {
                if c.command == octx.command {
                    CmdContext {
                        command: c.command.clone(),
                        args: c.args.clone(),
                        metadata: octx.metadata.union(&c.metadata).cloned().collect(),
                    }
                } else {
                    octx.clone()
                }
            })
        })
        .collect();

    if manifest.manifest.is_empty() || ctxs.is_empty() {
        create_prjfmt_manifest(prjfmt_toml, cache_dir, old_ctx)?;
    } else {
        println!("Format successful");
        println!("capturing formatted file's state...");
        create_prjfmt_manifest(prjfmt_toml, cache_dir, new_ctx)?;
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
                emoji::ERROR
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
                args: config.args.clone().unwrap_or_default(),
                metadata: path_to_filemeta(list_files)?,
            })
        })
        .collect::<Result<Vec<CmdContext>, Error>>()?;
    Ok(cmd_context)
}

/// prjfmt.toml structure
#[derive(Debug, Deserialize)]
pub struct Root {
    /// Map of formatters into the config
    pub formatters: BTreeMap<String, FmtConfig>,
}

/// Config for each formatters
#[derive(Debug, Deserialize)]
pub struct FmtConfig {
    /// File extensions that want to be formatted
    pub files: FileExtensions,
    /// File or Folder that is included to be formatted
    pub includes: Option<Vec<String>>,
    /// File or Folder that is excluded to be formatted
    pub excludes: Option<Vec<String>>,
    /// Command formatter to run
    pub command: Option<String>,
    /// Argument for formatter
    pub args: Option<Vec<String>>,
}

/// File extensions can be single string (e.g. "*.hs") or
/// list of string (e.g. [ "*.hs", "*.rs" ])
#[derive(Debug, Deserialize, Clone)]
#[serde(untagged)]
pub enum FileExtensions {
    /// Single file type
    SingleFile(String),
    /// List of file type
    MultipleFile(Vec<String>),
}

impl<'a> IntoIterator for &'a FileExtensions {
    type Item = &'a String;
    type IntoIter = either::Either<std::iter::Once<&'a String>, std::slice::Iter<'a, String>>;

    fn into_iter(self) -> Self::IntoIter {
        match self {
            FileExtensions::SingleFile(glob) => either::Either::Left(std::iter::once(glob)),
            FileExtensions::MultipleFile(globs) => either::Either::Right(globs.iter()),
        }
    }
}

#[derive(Debug, Deserialize, Serialize, Clone)]
/// Each context of the formatter config
pub struct CmdContext {
    /// formatter command to run
    pub command: String,
    /// formatter arguments or flags
    pub args: Vec<String>,
    /// formatter target path
    pub metadata: BTreeSet<FileMeta>,
}

#[derive(Debug, Deserialize, Serialize, Clone)]
/// File metadata created after the first prjfmt run
pub struct FileMeta {
    /// Last modification time listed in the file's metadata
    pub mtimes: i64,
    /// Path to the formatted file
    pub path: PathBuf,
}

impl Ord for FileMeta {
    fn cmp(&self, other: &Self) -> Ordering {
        if self.eq(other) {
            return Ordering::Equal;
        }
        if self.mtimes.eq(&other.mtimes) {
            return self.path.cmp(&other.path);
        }
        self.mtimes.cmp(&other.mtimes)
    }
}

impl PartialOrd for FileMeta {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl PartialEq for FileMeta {
    fn eq(&self, other: &Self) -> bool {
        self.mtimes == other.mtimes && self.path == other.path
    }
}

impl Eq for FileMeta {}
