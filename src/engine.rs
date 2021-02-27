//! The main formatting engine logic is in this module.

use crate::{config, eval_cache::CacheManifest, formatter::FormatterName, CLOG};
use crate::{expand_path, formatter::Formatter, get_meta_mtime, get_path_mtime, Mtime};
use ignore::WalkBuilder;
use std::iter::Iterator;
use std::path::{Path, PathBuf};
use std::{collections::BTreeMap, time::Instant};

/// Run the treefmt
pub fn run_treefmt(
    work_dir: &Path,
    cache_dir: &Path,
    treefmt_toml: &Path,
    paths: &[PathBuf],
    clear_cache: bool,
) -> anyhow::Result<()> {
    assert!(work_dir.is_absolute());
    assert!(cache_dir.is_absolute());
    assert!(treefmt_toml.is_absolute());

    let start_time = Instant::now();
    let mut traversed_files: usize = 0;
    let mut matched_files: usize = 0;
    let mut filtered_files: usize = 0;
    let mut reformatted_files: usize = 0;

    // unwrap: since the file must be in a folder, the parent path must exist
    let tree_root = treefmt_toml.parent().unwrap().to_path_buf();

    // Make sure all the given paths are absolute. Ignore the ones that point outside of the project root.
    let paths = paths.iter().fold(vec![], |mut sum, path| {
        let abs_path = expand_path(path, work_dir);
        if abs_path.starts_with(&tree_root) {
            sum.push(abs_path);
        } else {
            CLOG.warn(&format!(
                "Ignoring path {}, it is not in the project root",
                path.display()
            ));
        }
        sum
    });

    // Let's check that there is at least one path to format.
    if paths.is_empty() {
        CLOG.warn(&"Aborting, no paths to format".to_string());
        return Ok(());
    }

    // Load the treefmt.toml file
    let project_config = config::from_path(&treefmt_toml)?;

    CLOG.debug(&format!("load config: {:?}", start_time.elapsed()));

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

    CLOG.debug(&format!("load formatters: {:?}", start_time.elapsed()));

    // Load the eval cache
    let cache = if clear_cache {
        // Start with an empty cache
        CacheManifest::default()
    } else {
        CacheManifest::load(&cache_dir, &treefmt_toml)
    };
    CLOG.debug(&format!("load cache: {:?}", start_time.elapsed()));
    // Insert the new formatter configs
    let cache = cache.update_formatters(formatters.clone());

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
    let mut matches: BTreeMap<FormatterName, BTreeMap<PathBuf, Mtime>> = BTreeMap::new();

    // Now traverse the filesystem and classify each file. We also want the file mtime to see if it changed
    // afterwards.
    for walk_entry in walker {
        match walk_entry {
            Ok(dir_entry) => {
                if let Some(file_type) = dir_entry.file_type() {
                    if !file_type.is_dir() {
                        // Keep track of how many files were traversed
                        traversed_files += 1;

                        let path = dir_entry.path().to_path_buf();
                        // FIXME: complain if multiple matchers match the same path.
                        for (_, fmt) in formatters.clone() {
                            if fmt.clone().is_match(&path) {
                                // Keep track of how many files were associated with a formatter
                                matched_files += 1;

                                // unwrap: since the file exists, we assume that the metadata is also available
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
    CLOG.debug(&format!("tree walk: {:?}", start_time.elapsed()));

    // Filter out all of the paths that were already in the cache
    let matches = cache.clone().filter_matches(matches);

    CLOG.debug(&format!("filter_matches: {:?}", start_time.elapsed()));

    // Start another collection of formatter names to path to mtime.
    //
    // This time to collect the new paths that have been formatted.
    let mut new_matches: BTreeMap<FormatterName, BTreeMap<PathBuf, Mtime>> = BTreeMap::new();

    // Now run all the formatters and collect the formatted paths
    // TODO: do this in parallel
    for (formatter_name, path_mtime) in matches.clone() {
        let paths: Vec<PathBuf> = path_mtime.keys().cloned().collect();
        // unwrap: the key exists since matches was built from that previous collection
        let formatter = formatters.get(&formatter_name).unwrap();

        if !paths.is_empty() {
            // Keep track of the paths that are actually going to be formatted
            filtered_files += paths.len();

            let start_time = Instant::now();

            match formatter.clone().fmt(&paths.clone()) {
                // FIXME: do we care about the output?
                Ok(_) => {
                    CLOG.info(&format!(
                        "{}: {} files formatted in {:?}",
                        formatter.name,
                        paths.len(),
                        start_time.elapsed()
                    ));

                    // Get the new mtimes and compare them to the original ones
                    let new_paths = paths.into_iter().fold(BTreeMap::new(), |mut sum, path| {
                        // unwrap: assume that the file still exists after formatting
                        let mtime = get_path_mtime(&path).unwrap();
                        sum.insert(path, mtime);
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
    }
    CLOG.debug(&format!("format: {:?}", start_time.elapsed()));

    // Record the new matches in the cache
    let cache = cache.add_results(new_matches.clone());
    // And write to disk
    cache.write(cache_dir, treefmt_toml);
    CLOG.debug(&format!("write cache: {:?}", start_time.elapsed()));

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

    // Finally display all the paths that have been formatted
    for (_name, paths) in changed_matches {
        // Keep track of how many files were reformatted
        reformatted_files += paths.len();
        // println!("{}:", name);
        // for path in paths {
        //     println!("- {}", path.display());
        // }
    }

    println!(
        r#"
traversed {} files
matched {} files to formatters
left with {} files after cache
of whom {} files were re-formatted
all of this in {:?}
        "#,
        traversed_files,
        matched_files,
        filtered_files,
        reformatted_files,
        start_time.elapsed()
    );

    Ok(())
}
