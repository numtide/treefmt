//! The main formatting engine logic is in this module.

use crate::{config, eval_cache::CacheManifest, formatter::FormatterName};
use crate::{expand_path, formatter::Formatter, get_meta, get_path_meta, FileMeta};
use anyhow::anyhow;
use ignore::WalkBuilder;
use log::{debug, error, info, warn};
use rayon::prelude::*;
use std::io::{self, Write};
use std::iter::Iterator;
use std::path::{Path, PathBuf};
use std::{collections::BTreeMap, time::Instant};

/// Controls how the information is displayed at the end of a run
pub enum DisplayType {
    /// Just display some numbers
    Summary,
    /// Display the list of files that were affected
    Long,
}

/// Run the treefmt
pub fn run_treefmt(
    tree_root: &Path,
    work_dir: &Path,
    cache_dir: &Path,
    treefmt_toml: &Path,
    paths: &[PathBuf],
    no_cache: bool,
    clear_cache: bool,
    fail_on_change: bool,
) -> anyhow::Result<()> {
    assert!(tree_root.is_absolute());
    assert!(work_dir.is_absolute());
    assert!(cache_dir.is_absolute());
    assert!(treefmt_toml.is_absolute());

    let start_time = Instant::now();
    let mut phase_time = Instant::now();
    let mut timed_debug = |description: &str| {
        let now = Instant::now();
        debug!(
            "{}: {:.2?} (Δ {:.2?})",
            description,
            start_time.elapsed(),
            now.saturating_duration_since(phase_time)
        );
        phase_time = now;
    };

    let mut traversed_files: usize = 0;
    let mut matched_files: usize = 0;

    // Make sure all the given paths are absolute. Ignore the ones that point outside of the project root.
    let paths = paths.iter().fold(vec![], |mut sum, path| {
        let abs_path = expand_path(path, work_dir);
        if abs_path.starts_with(&tree_root) {
            sum.push(abs_path);
        } else {
            warn!(
                "Ignoring path {}, it is not in the project root",
                path.display()
            );
        }
        sum
    });

    // Let's check that there is at least one path to format.
    if paths.is_empty() {
        warn!("Aborting, no paths to format");
        return Ok(());
    }

    // Load the treefmt.toml file
    let project_config = config::from_path(treefmt_toml)?;

    let global_excludes = project_config
        .global
        .map(|g| g.excludes)
        .unwrap_or_default();

    timed_debug("load config");

    // Load all the formatter instances from the config. Ignore the ones that failed.
    let formatters = project_config.formatter.into_iter().fold(
        BTreeMap::new(),
        |mut sum, (name, mut fmt_config)| {
            fmt_config.excludes.extend_from_slice(&global_excludes);
            match Formatter::from_config(tree_root, &name, &fmt_config) {
                Ok(fmt_matcher) => {
                    sum.insert(fmt_matcher.name.clone(), fmt_matcher);
                }
                Err(err) => error!("Ignoring formatter #{} due to error: {}", name, err),
            };
            sum
        },
    );

    timed_debug("load formatters");

    // Load the eval cache
    let mut cache = if no_cache || clear_cache {
        // Start with an empty cache
        CacheManifest::default()
    } else {
        CacheManifest::load(cache_dir, treefmt_toml)
    };
    timed_debug("load cache");

    if !no_cache {
        // Insert the new formatter configs
        cache.update_formatters(formatters.clone());
    }

    // Configure the tree walker
    let walker = {
        // For some reason the WalkBuilder must start with one path, but can add more paths later.
        // unwrap: we checked before that there is at least one path in the vector
        let mut builder = WalkBuilder::new(paths.first().unwrap());
        // Add the other paths
        for path in paths[1..].iter() {
            builder.add(path);
        }
        // TODO: builder has a lot of interesting options.
        // TODO: use build_parallel with a Visitor.
        //       See https://docs.rs/ignore/0.4.17/ignore/struct.WalkParallel.html#method.visit
        builder.build()
    };

    // Start a collection of formatter names to path to mtime
    let mut matches: BTreeMap<FormatterName, BTreeMap<PathBuf, FileMeta>> = BTreeMap::new();

    // Now traverse the filesystem and classify each file. We also want the file mtime to see if it changed
    // afterwards.
    for walk_entry in walker {
        match walk_entry {
            Ok(dir_entry) => {
                if let Some(file_type) = dir_entry.file_type() {
                    // Ignore folders and symlinks. We don't want to format files outside the
                    // directory, and if the symlink destination is in the repo, it'll be matched
                    // when iterating over it.
                    if !file_type.is_dir() && !file_type.is_symlink() {
                        // Keep track of how many files were traversed
                        traversed_files += 1;

                        let path = dir_entry.path().to_path_buf();
                        // FIXME: complain if multiple matchers match the same path.
                        for (_, fmt) in formatters.clone() {
                            if fmt.clone().is_match(&path) {
                                // Keep track of how many files were associated with a formatter
                                matched_files += 1;

                                // unwrap: since the file exists, we assume that the metadata is also available
                                let mtime = get_meta(&dir_entry.metadata().unwrap());

                                matches
                                    .entry(fmt.name)
                                    .or_insert_with(BTreeMap::new)
                                    .insert(path.clone(), mtime);
                            }
                        }
                    }
                } else {
                    warn!("Couldn't get file type for {:?}", dir_entry.path())
                }
            }
            Err(err) => {
                warn!("traversal error: {}", err);
            }
        }
    }
    timed_debug("tree walk");

    // Filter out all of the paths that were already in the cache
    let matches = if !no_cache {
        cache.filter_matches(matches)
    } else {
        matches
    };

    timed_debug("filter_matches");

    // Keep track of the paths that are actually going to be formatted
    let filtered_files: usize = matches.values().map(|x| x.len()).sum();

    // Now run all the formatters and collect the formatted paths.
    let new_matches: BTreeMap<FormatterName, BTreeMap<PathBuf, FileMeta>> = matches
        .par_iter()
        .map(|(formatter_name, path_mtime)| {
            let paths: Vec<PathBuf> = path_mtime.keys().cloned().collect();
            // unwrap: the key exists since matches was built from that previous collection
            let formatter = formatters.get(formatter_name).unwrap();

            // Don't run the formatter if there are no paths to format!
            if paths.is_empty() {
                Ok((formatter_name.clone(), path_mtime.clone()))
            } else {
                let start_time = Instant::now();

                // Run the formatter
                for path_chunks in paths.chunks(1024) {
                    formatter.clone().fmt(path_chunks)?;
                }

                // Get the new mtimes and compare them to the original ones
                let new_paths = paths
                    .clone()
                    .into_iter()
                    .fold(BTreeMap::new(), |mut sum, path| {
                        // unwrap: assume that the file still exists after formatting
                        let mtime = get_path_meta(&path).unwrap();
                        sum.insert(path, mtime);
                        sum
                    });

                info!(
                    "{}: {} files processed in {:.2?}",
                    formatter.name,
                    paths.len(),
                    start_time.elapsed()
                );

                // Return the new mtimes
                Ok((formatter_name.clone(), new_paths))
            }
        })
        .collect::<anyhow::Result<BTreeMap<FormatterName, BTreeMap<PathBuf, FileMeta>>>>()?;
    timed_debug("format");

    if !no_cache {
        // Record the new matches in the cache
        cache.add_results(new_matches.clone());
        // And write to disk
        cache.write(cache_dir, treefmt_toml);
        timed_debug("write cache");
    }

    // Diff the old matches with the new matches
    let changed_matches: BTreeMap<FormatterName, Vec<PathBuf>> =
        new_matches
            .into_iter()
            .fold(BTreeMap::new(), |mut sum, (name, new_paths)| {
                // unwrap: we know that the name exists
                let old_paths = matches.get(&name).unwrap().clone();
                let filtered = new_paths
                    .iter()
                    .filter_map(|(k, v)| {
                        // unwrap: we know that the key exists
                        if old_paths.get(k).unwrap() == v {
                            None
                        } else {
                            Some(k.clone())
                        }
                    })
                    .collect();

                sum.insert(name, filtered);
                sum
            });

    // Get how many files were reformatted.
    let reformatted_files: usize = changed_matches.values().map(|x| x.len()).sum();
    // TODO: this will be configurable by the user in the future.
    let mut display_type = DisplayType::Summary;
    let mut ret: anyhow::Result<()> = Ok(());

    // Fail if --fail-on-change was passed.
    if reformatted_files > 0 && fail_on_change {
        // Switch the display type to long
        display_type = DisplayType::Long;
        ret = Err(anyhow!("fail-on-change"))
    }

    match display_type {
        DisplayType::Summary => {
            print_summary(
                traversed_files,
                matched_files,
                filtered_files,
                reformatted_files,
                start_time,
            );
        }
        DisplayType::Long => {
            print_summary(
                traversed_files,
                matched_files,
                filtered_files,
                reformatted_files,
                start_time,
            );
            println!("\nformatted files:");
            for (name, paths) in changed_matches {
                if !paths.is_empty() {
                    println!("{}:", name);
                    for path in paths {
                        println!("- {}", path.display());
                    }
                }
            }
        }
    }

    ret
}

fn print_summary(
    traversed_files: usize,
    matched_files: usize,
    filtered_files: usize,
    reformatted_files: usize,
    start_time: std::time::Instant,
) {
    println!(
        r#"
traversed {} files
matched {} files to formatters
left with {} files after cache
of whom {} files were re-formatted
all of this in {:.0?}
        "#,
        traversed_files,
        matched_files,
        filtered_files,
        reformatted_files,
        start_time.elapsed()
    );
}

/// Run the treefmt in a stdin buffer, and print it out back to stdout
pub fn run_treefmt_stdin(
    tree_root: &Path,
    work_dir: &Path,
    cache_dir: &Path,
    treefmt_toml: &Path,
    path: &Path,
) -> anyhow::Result<()> {
    assert!(tree_root.is_absolute());
    assert!(work_dir.is_absolute());
    assert!(cache_dir.is_absolute());
    assert!(treefmt_toml.is_absolute());
    assert!(path.is_absolute());

    // Make sure all the given paths are absolute. Ignore the ones that point outside of the project root.
    if !path.starts_with(&tree_root) {
        anyhow!(
            "Ignoring path {}, it is not in the project root",
            path.display()
        );
    };

    // Load the treefmt.toml file
    let project_config = config::from_path(treefmt_toml)?;

    let global_excludes = project_config
        .global
        .map(|g| g.excludes)
        .unwrap_or_default();

    // Load all the formatter instances from the config. Ignore the ones that failed.
    let formatters = project_config.formatter.into_iter().fold(
        BTreeMap::new(),
        |mut sum, (name, mut fmt_config)| {
            fmt_config.excludes.extend_from_slice(&global_excludes);
            match Formatter::from_config(tree_root, &name, &fmt_config) {
                Ok(fmt_matcher) => {
                    sum.insert(fmt_matcher.name.clone(), fmt_matcher);
                }
                Err(err) => error!("Ignoring formatter #{} due to error: {}", name, err),
            };
            sum
        },
    );

    // Collect all formatters that match the path
    let formatters: Vec<&Formatter> = formatters.values().filter(|f| f.is_match(&path)).collect();

    if formatters.is_empty() {
        warn!("no formatter found for path {:?}", path);
        // Just copy stdin to stdout
        io::copy(&mut io::stdin().lock(), &mut io::stdout().lock())?;
        return Ok(()); // Nothing more to do here
    } else if formatters.len() > 1 {
        warn!("multiple formatters matched the path. picking the first one");
    } else {
        info!("running {}", formatters.first().unwrap().name)
    }

    // Construct a "unique" filename. We want the code formatter to recognise the file type so it has the same extension.
    // And it has to live in the project's folder.
    //
    // NOTE: in some conditions like SIGINT and panic, the tmpfile won't be cleaned.
    let mut tmpfile = tempfile::Builder::new()
        .prefix("_tmp")
        .suffix(&path.file_name().unwrap())
        .tempfile_in(path.parent().unwrap())?;

    // Wrap this in a closure to control the error flow.
    let mut run = || -> anyhow::Result<()> {
        // Copy stdin to the file
        io::copy(&mut io::stdin().lock(), &mut tmpfile)?;

        // Make sure the file content is written to disk.
        tmpfile.flush()?;

        // Now that the file has been written, invoke the formatter.
        formatters
            .first()
            .unwrap()
            .fmt(&[tmpfile.path().to_path_buf()])?;

        // Seek back to start
        let mut tmpfile = tmpfile.reopen()?;

        // Copy the file to stdout
        io::copy(&mut tmpfile, &mut io::stdout().lock())?;

        Ok(())
    };

    let ret = run();

    // Free the temporary file explicitly here
    tmpfile.close()?;

    ret
}
