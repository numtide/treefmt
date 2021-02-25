//! The main formatting engine logic is in this module.

use crate::{config, eval_cache::CacheManifest, formatter::FormatterName, CLOG};
use crate::{expand_path, formatter::Formatter, get_meta_mtime, get_path_mtime, Mtime};
use ignore::WalkBuilder;
use std::collections::BTreeMap;
use std::iter::Iterator;
use std::path::PathBuf;

/// Run the treefmt
pub fn run_treefmt(
    work_dir: &PathBuf,
    cache_dir: &PathBuf,
    treefmt_toml: &PathBuf,
    paths: &Vec<PathBuf>,
) -> anyhow::Result<()> {
    assert!(work_dir.is_absolute());
    assert!(cache_dir.is_absolute());
    assert!(treefmt_toml.is_absolute());

    let tree_root = treefmt_toml.parent().unwrap().to_path_buf();

    // Make sure all the given paths are absolute. Ignore the ones that point outside of the project root.
    let paths = paths.iter().fold(vec![], |mut sum, path| {
        let abs_path = expand_path(path, work_dir);
        if !abs_path.starts_with(&tree_root) {
            CLOG.warn(&format!(
                "Ignoring path {}, it is not in the project root",
                path.display()
            ));
        } else {
            sum.push(abs_path);
        }
        sum
    });

    // Let's check that there is at least one path to format.
    if paths.is_empty() {
        CLOG.warn(&format!("Aborting, no paths to format"));
        return Ok(());
    }

    // Load the treefmt.toml file
    let project_config = config::from_path(&treefmt_toml)?;

    // Load all the formatter instances from the config. Ignore the ones that failed.
    let formatters =
        project_config
            .formatter
            .iter()
            .fold(BTreeMap::new(), |mut sum, (name, fmt_config)| {
                match Formatter::from_config(&tree_root.clone(), &name, &fmt_config) {
                    Ok(fmt_matcher) => {
                        sum.insert(fmt_matcher.name.clone(), fmt_matcher);
                    }
                    Err(err) => CLOG.error(&format!(
                        "Ignoring formatter #{} due to error: {}",
                        name, err
                    )),
                };
                sum
            });

    // Load the eval cache
    let cache = CacheManifest::load(&cache_dir, &treefmt_toml);

    // Insert the new formatter configs
    let cache = cache.update_formatters(formatters.clone());

    // Configure the tree walker
    let walker = {
        // For some reason the WalkBuilder must start with one path, but can add more paths later.
        let mut builder = WalkBuilder::new(paths.first().unwrap());
        // Add the other paths
        for path in paths[1..].iter() {
            builder.add(path);
        }
        // TODO: builder has a lot of interesting options.
        builder.build()
    };

    // Start a collection of formatter names to path to mtime
    let mut matches: BTreeMap<FormatterName, BTreeMap<PathBuf, Mtime>> = BTreeMap::new();

    // Now traverse the filesystem and classify each file. We also want the file mtime to see if it changed
    // afterwards.
    for walk_entry in walker {
        match walk_entry {
            Ok(dir_entry) => {
                if let Some(file_type) = dir_entry.file_type() {
                    if !file_type.is_dir() {
                        let path = dir_entry.path().to_path_buf();
                        // FIXME: complain if multiple matchers match the same path.
                        for (_, fmt) in formatters.clone().into_iter() {
                            if fmt.clone().is_match(&path) {
                                let mtime = get_meta_mtime(&dir_entry.metadata().unwrap());

                                matches
                                    .entry(fmt.name)
                                    .or_insert_with(BTreeMap::new)
                                    .insert(path.clone(), mtime);
                            }
                        }
                    }
                } else {
                    CLOG.warn(&format!(
                        "Couldn't get file type for {:?}",
                        dir_entry.path()
                    ))
                }
            }
            Err(err) => {
                CLOG.warn(&format!("traversal error: {}", err));
            }
        }
    }

    // Filter out all of the paths that were already in the cache
    let matches = cache.clone().filter_matches(matches.clone());

    // Start another collection of formatter names to path to mtime.
    //
    // This time to collect the new paths that have been formatted.
    let mut new_matches: BTreeMap<FormatterName, BTreeMap<PathBuf, Mtime>> = BTreeMap::new();

    // Now run all the formatters and collect the formatted paths
    // TODO: do this in parallel
    for (formatter_name, path_mtime) in matches.clone().into_iter() {
        let paths: Vec<PathBuf> = path_mtime.keys().cloned().collect();
        let formatter = formatters.get(&formatter_name).unwrap();
        match formatter.clone().fmt(&paths.clone()) {
            // FIXME: do we care about the output?
            Ok(_) => {
                // Get the new mtimes and compare them to the original ones
                let new_paths = paths.into_iter().fold(BTreeMap::new(), |mut sum, path| {
                    let mtime = get_path_mtime(&path).unwrap();
                    sum.insert(path.clone(), mtime);
                    sum
                });
                new_matches.insert(formatter_name.clone(), new_paths);
            }
            Err(err) => {
                // FIXME: What is the right behaviour if a formatter has failed running?
                CLOG.error(&format!("{} failed: {}", &formatter, err));
            }
        }
    }

    // Record the new matches in the cache
    let cache = cache.add_results(new_matches.clone());
    // And write to disk
    cache.write(cache_dir, treefmt_toml);

    // Diff the old matches with the new matches
    let changed_matches: BTreeMap<FormatterName, Vec<PathBuf>> = new_matches
        .clone()
        .into_iter()
        .fold(BTreeMap::new(), |mut sum, (name, new_paths)| {
            let old_paths = matches.get(&name).unwrap().clone();
            let filtered = new_paths
                .clone()
                .iter()
                .filter_map(|(k, v)| {
                    if old_paths.get(k).unwrap() != v {
                        Some(k.clone())
                    } else {
                        None
                    }
                })
                .collect();

            sum.insert(name, filtered);
            sum
        });

    // Finally display all the paths that have been formatted
    for (name, paths) in changed_matches.into_iter() {
        println!("{}:", name);
        for path in paths {
            println!("- {}", path.display());
        }
    }

    Ok(())
}
