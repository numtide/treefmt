//! The main formatting engine logic is in this module.

use crate::config::Root;
use crate::{config, eval_cache::CacheManifest, formatter::FormatterName, Deserialize};
use crate::{expand_path, formatter::Formatter, get_meta, get_path_meta, FileMeta};
use anyhow::anyhow;
use ignore::{Walk, WalkBuilder};
use log::{debug, error, info, warn};
use rayon::prelude::*;
use std::fs::File;
use std::io::{self, Write};
use std::iter::Iterator;
use std::path::{Path, PathBuf};
use std::{collections::BTreeMap, time::Instant};
use watchman_client::prelude::*;

/// Controls how the information is displayed at the end of a run
pub enum DisplayType {
    /// Just display some numbers
    Summary,
    /// Display the list of files that were affected
    Long,
}

watchman_client::query_result_type! {
    struct WatchmanResponse {
        name: NameField,
        mtime: MTimeField,
        size: SizeField,
    }
}

/// Run the treefmt
#[allow(clippy::too_many_arguments)]
pub async fn run_treefmt(
    tree_root: &Path,
    work_dir: &Path,
    cache_dir: &Path,
    treefmt_toml: &Path,
    paths: &[PathBuf],
    hidden: bool,
    no_cache: bool,
    clear_cache: bool,
    fail_on_change: bool,
    allow_missing_formatter: bool,
    selected_formatters: &Option<Vec<String>>,
    watchman: &Option<&Client>,
) -> anyhow::Result<()> {
    assert!(tree_root.is_absolute());
    assert!(work_dir.is_absolute());
    assert!(cache_dir.is_absolute());
    assert!(treefmt_toml.is_absolute());
    let flavor = if watchman.is_some() {
        "watchman"
    } else {
        "stat"
    };

    let mut stats = Statistics::init();

    // Make sure all the given paths are absolute. Ignore the ones that point outside of the project root.
    let paths = paths.iter().fold(vec![], |mut sum, path| {
        let abs_path = expand_path(path, work_dir);
        if abs_path.starts_with(tree_root) {
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

    stats.timed_debug("load config");

    let formatters = load_formatters(
        project_config,
        tree_root,
        allow_missing_formatter,
        selected_formatters,
        &mut stats,
    )?;

    // Load the eval cache
    let mut cache = if no_cache || clear_cache {
        // Start with an empty cache
        CacheManifest::default()
    } else {
        CacheManifest::load(cache_dir, treefmt_toml, flavor)
    };
    stats.timed_debug("load cache");

    if !no_cache {
        // Insert the new formatter configs
        cache.update_formatters(formatters.clone());
    }
    stats.timed_debug("update formatters");
    let mut resolved_root: Option<ResolvedRoot> = None;

    let matches = match watchman {
        None => {
            let walker = build_walker(paths, hidden);

            let matches = collect_matches_from_walker(walker, &formatters, &mut stats);
            stats.timed_debug("tree walk");
            matches
        }
        Some(watchman) => {
            let root = watchman
                .resolve_root(CanonicalPath::with_canonicalized_path(
                    tree_root.to_path_buf(),
                ))
                .await
                .map_err(anyhow::Error::new)?;
            stats.timed_debug("resolve root");

            let mut matches: Vec<Expr> = vec![];

            for fmt in formatters.values() {
                let mut includes: Vec<Expr> = vec![];
                let mut excludes: Vec<Expr> = vec![];

                // As far as "watchman" concerned, "*.c" matches only $ROOT/foo.c, but not
                // $ROOT/src.foo.c
                for pattern in fmt.includes_str.iter() {
                    let mut s = pattern.to_string();
                    s.insert_str(0, "**/");

                    includes.push(Expr::Match(MatchTerm {
                        glob: s,
                        include_dot_files: hidden,
                        no_escape: false,
                        wholename: true,
                    }));
                }
                for pattern in fmt.excludes_str.iter() {
                    let mut s = pattern.to_string();
                    s.insert_str(0, "**/");

                    excludes.push(Expr::Match(MatchTerm {
                        glob: s,
                        include_dot_files: hidden,
                        no_escape: false,
                        wholename: true,
                    }));
                }
                if excludes.len() > 0 {
                    matches.push(Expr::All(vec![
                        Expr::Any(includes),
                        Expr::Not(Box::new(Expr::Any(excludes))),
                    ]));
                } else {
                    matches.push(Expr::Any(includes));
                }
            }

            let since = if no_cache {
                ClockSpec::null()
            } else {
                match cache.clock.clone() {
                    // FIXME: Avoid .clone()
                    Some(since) => since,
                    _ => ClockSpec::null(),
                }
            };

            let query = QueryRequestCommon {
                expression: Some(Expr::All(vec![
                    Expr::Exists,
                    Expr::FileType(FileType::Regular),
                    Expr::Since(SinceTerm::ObservedClock(since)),
                    Expr::Any(matches),
                ])),
                ..Default::default()
            };

            let resp: QueryResult<WatchmanResponse> = watchman
                .query(&root, query)
                .await
                .map_err(anyhow::Error::new)?;
            resolved_root = Some(root);
            let files = resp.files.unwrap_or_default();
            stats.timed_debug(&format!("watchman query (returned {} files)", files.len()));
            let mut matches: BTreeMap<FormatterName, BTreeMap<PathBuf, FileMeta>> =
                Default::default();

            // Now the data returned by "watchman" must be massaged into the shape used by
            // stat(2)-based branch of code. List of fields returned by "watchman" are files that
            // (1) are changed and (2) match some glob of some formatter, but I have to figure
            // myself to which formatter they belong. Also, "watchman" has no concept of exclusion
            // globs, so that also must be filtered out.
            //
            // In addition, "watchman" returns list of files modified without respect to
            // "paths" parameter, so that also must be post-processed.
            for r in files.iter() {
                let pathbuf = expand_path(&r.name.clone().into_inner(), tree_root); // FIXME: Get rid of clone().
                match formatters.values().find(|fmt| fmt.is_match(&pathbuf)) {
                    // Watchman is not aware of exclusion globs.
                    None => {}
                    Some(fmt) => {
                        let pathbuf = expand_path(&pathbuf, tree_root);
                        // File changed is somewhere under "paths".
                        if paths.iter().any(|path| pathbuf.starts_with(path)) {
                            let meta = FileMeta {
                                size: r.size.clone().into_inner(),
                                mtime: r.mtime.clone().into_inner(),
                            };
                            matches
                                .entry(fmt.name.clone())
                                .or_default()
                                .insert(pathbuf, meta);
                        }
                    }
                }
            }
            stats.timed_debug("post-process watchman list");
            matches
        }
    };
    stats.timed_debug("finish walking");

    // Filter out all of the paths that were already in the cache
    let matches = if !no_cache {
        cache.filter_matches(matches)
    } else {
        matches
    };

    stats.timed_debug("filter_matches");

    // Keep track of the paths that are actually going to be formatted
    let filtered_files: usize = matches.values().map(|x| x.len()).sum();
    stats.set_filtered_files(filtered_files);

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
    stats.timed_debug("format");

    if !no_cache {
        match watchman {
            None => cache.add_results(new_matches.clone()),
            // We only need "since" value in cache.
            Some(watchman) => {
                cache.clock = Some(
                    watchman
                        .clock(&resolved_root.unwrap(), Default::default())
                        .await?,
                );
            }
        }

        if !watchman.is_some() {
            cache.add_results(new_matches.clone());
        };
        // And write to disk
        cache.write(cache_dir, treefmt_toml, flavor);
        stats.timed_debug("write cache");
    }

    let changed_matches: BTreeMap<FormatterName, Vec<PathBuf>> =
        diff_matches(new_matches, matches, &mut stats);

    let mut ret: anyhow::Result<()> = Ok(());
    // Fail if --fail-on-change was passed.
    if stats.reformatted_files > 0 && fail_on_change {
        // Switch the display type to long
        // TODO: this will be configurable by the user in the future.
        stats.display = DisplayType::Long;
        ret = Err(anyhow!("fail-on-change"));
    }

    match stats.display {
        DisplayType::Summary => {
            stats.print_summary();
        }
        DisplayType::Long => {
            stats.print_summary();
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

/// Diff the old matches with the new matches
fn diff_matches(
    new_matches: BTreeMap<FormatterName, BTreeMap<PathBuf, FileMeta>>,
    matches: BTreeMap<FormatterName, BTreeMap<PathBuf, FileMeta>>,
    stats: &mut Statistics,
) -> BTreeMap<FormatterName, Vec<PathBuf>> {
    let diffed_matches =
        new_matches
            .into_iter()
            .fold(BTreeMap::new(), |mut sum, (name, new_paths)| {
                // unwrap: we know that the name exists
                let old_paths = matches.get(&name).unwrap().clone();
                let filtered: Vec<PathBuf> = new_paths
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
    let reformatted_files = diffed_matches.values().map(|x| x.len()).sum();
    stats.set_reformatted_files(reformatted_files);
    diffed_matches
}

/// Load all the formatter instances from the config.
/// Returns an error if a formatter is missing and not explicitly allowed.
fn load_formatters(
    root: Root,
    tree_root: &Path,
    allow_missing_formatter: bool,
    selected_formatters: &Option<Vec<String>>,
    stats: &mut Statistics,
) -> anyhow::Result<BTreeMap<FormatterName, Formatter>> {
    let mut expected_count = 0;
    let formatter = root.formatter;
    let global_excludes = root.global.map(|g| g.excludes).unwrap_or_default();
    let formatters =
        formatter
            .into_iter()
            .fold(BTreeMap::new(), |mut sum, (name, mut fmt_config)| {
                expected_count += 1;
                fmt_config.excludes.extend_from_slice(&global_excludes);
                match Formatter::from_config(tree_root, &name, &fmt_config) {
                    Ok(fmt_matcher) => match selected_formatters {
                        Some(f) => {
                            if f.contains(&name) {
                                sum.insert(fmt_matcher.name.clone(), fmt_matcher);
                            }
                        }
                        None => {
                            sum.insert(fmt_matcher.name.clone(), fmt_matcher);
                        }
                    },
                    Err(err) => {
                        if allow_missing_formatter {
                            error!("Ignoring formatter #{} due to error: {}", name, err)
                        } else {
                            error!("Failed to load formatter #{}: {}", name, err)
                        }
                    }
                };
                sum
            });

    stats.timed_debug("load formatters");
    // Check if the number of configured formatters matches the number of formatters loaded
    if !(allow_missing_formatter || formatters.len() == expected_count) {
        return Err(anyhow!("One or more formatters are missing"));
    }
    Ok(formatters)
}

/// Walk over the entries and collect it's matches.
/// The matches are a collection of formatter names to path to mtime.
/// We want the file mtime to see if it changed afterwards.
fn collect_matches_from_walker(
    walker: Walk,
    formatters: &BTreeMap<FormatterName, Formatter>,
    stats: &mut Statistics,
) -> BTreeMap<FormatterName, BTreeMap<PathBuf, FileMeta>> {
    let mut matches = BTreeMap::new();

    for walk_entry in walker {
        match walk_entry {
            Ok(dir_entry) => {
                if let Some(file_type) = dir_entry.file_type() {
                    // Ignore folders and symlinks. We don't want to format files outside the
                    // directory, and if the symlink destination is in the repo, it'll be matched
                    // when iterating over it.
                    if !file_type.is_dir() && !file_type.is_symlink() {
                        stats.traversed_file();

                        let path = dir_entry.path().to_path_buf();
                        // FIXME: complain if multiple matchers match the same path.
                        for (_, fmt) in formatters.clone() {
                            if fmt.clone().is_match(&path) {
                                stats.matched_file();

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
    matches
}

/// Configure and build the tree walker
fn build_walker(paths: Vec<PathBuf>, hidden: bool) -> Walk {
    // For some reason the WalkBuilder must start with one path, but can add more paths later.
    // unwrap: we checked before that there is at least one path in the vector
    let mut builder = WalkBuilder::new(paths.first().unwrap());
    // Add the other paths
    for path in paths[1..].iter() {
        builder.add(path);
    }
    builder.hidden(!hidden);
    // TODO: builder has a lot of interesting options.
    // TODO: use build_parallel with a Visitor.
    //       See https://docs.rs/ignore/0.4.17/ignore/struct.WalkParallel.html#method.visit
    builder.build()
}

/// Run the treefmt in a stdin buffer, and print it out back to stdout
pub fn run_treefmt_stdin(
    tree_root: &Path,
    work_dir: &Path,
    cache_dir: &Path,
    treefmt_toml: &Path,
    path: &Path,
    selected_formatters: &Option<Vec<String>>,
) -> anyhow::Result<()> {
    assert!(tree_root.is_absolute());
    assert!(work_dir.is_absolute());
    assert!(cache_dir.is_absolute());
    assert!(treefmt_toml.is_absolute());
    assert!(path.is_absolute());

    // Make sure all the given paths are absolute. Ignore the ones that point outside of the project root.
    if !path.starts_with(tree_root) {
        return Err(anyhow!(
            "Ignoring path {}, it is not in the project root",
            path.display()
        ));
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
                Ok(fmt_matcher) => match selected_formatters {
                    Some(f) => {
                        if f.contains(&name) {
                            sum.insert(fmt_matcher.name.clone(), fmt_matcher);
                        }
                    }
                    None => {
                        sum.insert(fmt_matcher.name.clone(), fmt_matcher);
                    }
                },
                Err(err) => error!("Ignoring formatter #{} due to error: {}", name, err),
            };
            sum
        },
    );

    // Collect all formatters that match the path
    let formatters: Vec<&Formatter> = formatters.values().filter(|f| f.is_match(path)).collect();

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
        let mut tmpfile = File::open(tmpfile.path())?;

        // Copy the file to stdout
        io::copy(&mut tmpfile, &mut io::stdout().lock())?;

        Ok(())
    };

    let ret = run();

    // Free the temporary file explicitly here
    tmpfile.close()?;

    ret
}

struct Statistics {
    display: DisplayType,
    start_time: Instant,
    phase_time: Instant,
    traversed_files: usize,
    matched_files: usize,
    filtered_files: usize,
    reformatted_files: usize,
}

impl Statistics {
    /// Initialize the timer
    fn init() -> Self {
        let start_time = Instant::now();
        let phase_time = Instant::now();
        Self {
            display: DisplayType::Summary,
            start_time,
            phase_time,
            traversed_files: 0,
            matched_files: 0,
            filtered_files: 0,
            reformatted_files: 0,
        }
    }
    /// Keep track of how many files were traversed
    fn traversed_file(&mut self) {
        self.traversed_files += 1;
    }
    /// Keep track of how many files were associated with a formatter
    fn matched_file(&mut self) {
        self.matched_files += 1;
    }
    fn timed_debug(&mut self, description: &str) {
        let now = Instant::now();
        debug!(
            "{}: {:.2?} (Δ {:.2?})",
            description,
            self.start_time.elapsed(),
            now.saturating_duration_since(self.phase_time)
        );
        self.phase_time = now;
    }

    fn set_filtered_files(&mut self, filtered_files: usize) {
        self.filtered_files = filtered_files;
    }

    fn set_reformatted_files(&mut self, reformatted_files: usize) {
        self.reformatted_files = reformatted_files;
    }
    fn print_summary(&self) {
        println!(
            "{} files changed in {:.0?} (found {}, matched {}, cache misses {})",
            self.reformatted_files,
            self.start_time.elapsed(),
            self.traversed_files,
            self.matched_files,
            self.filtered_files,
        );
    }
}

#[cfg(test)]
mod tests {
    use crate::config::from_string;

    use super::*;

    pub mod utils {
        use std::fs::{self, File};
        use std::io::Write;
        use std::os::unix::prelude::OpenOptionsExt;
        use std::path::{Path, PathBuf};

        use tempfile::TempDir;

        pub fn tmp_mkdir() -> TempDir {
            tempfile::tempdir().unwrap()
        }
        pub fn mkdir<P>(path: P)
        where
            P: AsRef<Path>,
        {
            fs::create_dir_all(path).unwrap();
        }
        pub fn write_file<P>(path: P, stream: &str)
        where
            P: AsRef<Path>,
        {
            let mut file = File::create(path).unwrap();
            file.write_all(stream.as_bytes()).unwrap();
        }
        pub fn write_binary_file<P>(path: P, stream: &str)
        where
            P: AsRef<Path>,
        {
            let mut file = fs::OpenOptions::new()
                .create(true)
                .write(true)
                .mode(0o770)
                .open(path)
                .unwrap();
            file.write_all(stream.as_bytes()).unwrap();
        }

        pub struct Git<'a> {
            path: PathBuf,
            exclude: Option<&'a str>,
            ignore: Option<&'a str>,
            git_ignore: Option<&'a str>,
            /// Whether to write into `.git` directory.
            /// Useful in case testing outside of git is desired.
            write_git: bool,
        }

        impl<'a> Git<'a> {
            pub fn new(root: PathBuf) -> Self {
                Self {
                    path: root,
                    exclude: None,
                    ignore: None,
                    git_ignore: None,
                    write_git: true,
                }
            }
            pub fn exclude(&mut self, content: &'a str) -> &mut Self {
                self.exclude = Some(content);
                self
            }
            pub fn ignore(&mut self, content: &'a str) -> &mut Self {
                self.ignore = Some(content);
                self
            }
            pub fn git_ignore(&mut self, content: &'a str) -> &mut Self {
                self.ignore = Some(content);
                self
            }
            pub fn write_git(&mut self, write_git: bool) -> &mut Self {
                self.write_git = write_git;
                self
            }
            /// Creates all configured directories and files.
            pub fn create(&mut self) {
                let git_dir = self.path.join(".git");
                if self.write_git {
                    mkdir(&git_dir);
                }
                if let Some(exclude) = self.exclude {
                    if !self.write_git {
                        panic!(
                            "Can't write git specific personal excludes without a .git directory."
                        )
                    }
                    let info_dir = git_dir.join("info");
                    mkdir(&info_dir);
                    let exclude_file = info_dir.join("exclude");
                    write_file(&exclude_file, exclude);
                }
                if let Some(ignore) = self.ignore {
                    let ignore_file = self.path.join(".ignore");
                    write_file(&ignore_file, ignore);
                }
                if let Some(gitignore) = self.git_ignore {
                    let ignore_file = self.path.join(".gitignore");
                    write_file(&ignore_file, gitignore);
                }
            }
        }
    }

    #[test]
    fn test_diff_matches_no_changes_no_files() {
        let new_matches = BTreeMap::new();
        let matches = BTreeMap::new();
        let mut stats = Statistics::init();

        let result = diff_matches(new_matches, matches, &mut stats);

        assert_eq!(result.len(), 0);
        assert_eq!(stats.reformatted_files, 0);
    }
    #[test]
    fn test_diff_matches_no_changes() {
        let mk_file_meta = |mtime, size| FileMeta { mtime, size };
        let mut new_matches = BTreeMap::new();
        let mut matches = BTreeMap::new();
        let mut stats = Statistics::init();

        let metadata = [
            mk_file_meta(0, 0),
            mk_file_meta(1, 1),
            mk_file_meta(2, 2),
            mk_file_meta(3, 3),
        ];
        let files = ["test", "test1", "test2", "test3"];
        let file_metadata: BTreeMap<_, _> = metadata
            .iter()
            .zip(files.iter())
            .map(|(meta, name)| (PathBuf::from(name), *meta))
            .collect();

        new_matches.insert(FormatterName::new("gofmt"), file_metadata.clone());
        new_matches.insert(FormatterName::new("rustfmt"), file_metadata.clone());
        matches.insert(FormatterName::new("gofmt"), file_metadata.clone());
        matches.insert(FormatterName::new("rustfmt"), file_metadata);

        let result = diff_matches(new_matches, matches, &mut stats);

        assert_eq!(result.len(), 2);
        assert_eq!(stats.reformatted_files, 0);
    }

    #[test]
    fn test_diff_matches_with_changes() {
        let mk_file_meta = |mtime, size| FileMeta { mtime, size };
        let mut new_matches = BTreeMap::new();
        let mut matches = BTreeMap::new();
        let mut stats = Statistics::init();

        let metadata = [
            mk_file_meta(0, 0),
            mk_file_meta(1, 1),
            mk_file_meta(2, 2),
            mk_file_meta(3, 3),
        ];
        let metadata_gofmt = [
            mk_file_meta(0, 0),
            mk_file_meta(2, 1), // Changed
            mk_file_meta(2, 2),
            mk_file_meta(5, 3), // Changed
        ];
        let metadata_rustfmt = [
            mk_file_meta(0, 0),
            mk_file_meta(1, 1),
            mk_file_meta(3, 2), // Changed
            mk_file_meta(3, 3),
        ];
        let files = ["test", "test1", "test2", "test3"];
        let file_metadata: BTreeMap<_, _> = metadata
            .iter()
            .zip(files.iter())
            .map(|(meta, name)| (PathBuf::from(name), *meta))
            .collect();
        let file_metadata_gofmt: BTreeMap<_, _> = metadata_gofmt
            .iter()
            .zip(files.iter())
            .map(|(meta, name)| (PathBuf::from(name), *meta))
            .collect();
        let file_metadata_rusftfmt: BTreeMap<_, _> = metadata_rustfmt
            .iter()
            .zip(files.iter())
            .map(|(meta, name)| (PathBuf::from(name), *meta))
            .collect();

        new_matches.insert(FormatterName::new("gofmt"), file_metadata.clone());
        new_matches.insert(FormatterName::new("rustfmt"), file_metadata);
        matches.insert(FormatterName::new("gofmt"), file_metadata_gofmt);
        matches.insert(FormatterName::new("rustfmt"), file_metadata_rusftfmt);

        let result = diff_matches(new_matches, matches, &mut stats);

        assert_eq!(result.len(), 2);
        assert_eq!(stats.reformatted_files, 3);
    }

    #[test]
    fn test_formatter_loading_some() {
        let tmpdir = utils::tmp_mkdir();

        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        utils::write_binary_file(&black, " ");
        utils::write_binary_file(&nixpkgs_fmt, " ");

        let config = format!(
            "
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\"]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]
        "
        );

        let root = from_string(&config).unwrap();
        let tree_root = tmpdir.path();

        let selected_formatters = None;
        let allow_missing_formatter = false;
        let mut stats = Statistics::init();

        let formatters = load_formatters(
            root,
            tree_root,
            allow_missing_formatter,
            &selected_formatters,
            &mut stats,
        )
        .unwrap();

        assert_eq!(formatters.len(), 2);
    }
    #[test]
    #[should_panic]
    fn test_formatter_loading_some_missing_formatter() {
        let tmpdir = utils::tmp_mkdir();

        // black is the missing formatter here
        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        utils::write_binary_file(&nixpkgs_fmt, " ");

        let config = format!(
            "
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\"]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]
        "
        );

        let root = from_string(&config).unwrap();
        let tree_root = tmpdir.path();

        let selected_formatters = None;
        let allow_missing_formatter = false;
        let mut stats = Statistics::init();

        load_formatters(
            root,
            tree_root,
            allow_missing_formatter,
            &selected_formatters,
            &mut stats,
        )
        .unwrap();
    }
    #[test]
    fn test_formatter_loading_some_missing_formatter_allowed() {
        let tmpdir = utils::tmp_mkdir();

        // black is the missing formatter here
        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        utils::write_binary_file(&nixpkgs_fmt, " ");

        let config = format!(
            "
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\"]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]
        "
        );

        let root = from_string(&config).unwrap();
        let tree_root = tmpdir.path();

        let selected_formatters = None;
        let allow_missing_formatter = true;
        let mut stats = Statistics::init();

        let formatters = load_formatters(
            root,
            tree_root,
            allow_missing_formatter,
            &selected_formatters,
            &mut stats,
        )
        .unwrap();
        assert_eq!(formatters.len(), 1);
    }
    #[test]
    #[should_panic]
    fn test_formatter_loading_missing_includes() {
        let tmpdir = utils::tmp_mkdir();

        let config = r#"
        [formatter.python]
        command = "black"
        includes = ["*.py"]

        [formatter.nix]
        command = "nixpkgs-fmt"
        "#;

        let root = from_string(config).unwrap();
        let tree_root = tmpdir.path();

        let selected_formatters = None;
        let allow_missing_formatter = false;
        let mut stats = Statistics::init();

        load_formatters(
            root,
            tree_root,
            allow_missing_formatter,
            &selected_formatters,
            &mut stats,
        )
        .unwrap();
    }
    #[test]
    fn test_formatter_loading_selected_allow_missing() {
        let tmpdir = utils::tmp_mkdir();

        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        let elm_fmt = tmpdir.path().join("elm-fmt");
        utils::write_binary_file(&black, " ");
        utils::write_binary_file(&nixpkgs_fmt, " ");
        utils::write_binary_file(&elm_fmt, " ");

        let config = format!(
            "
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\"]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]

        [formatter.elm]
        command = {elm_fmt:?}
        options = [\"--yes\"]
        includes = [\"*.elm\"]
        "
        );

        let root = from_string(&config).unwrap();
        let tree_root = tmpdir.path();

        let selected_formatters = Some(vec!["python".into(), "nix".into(), "gofmt".into()]);
        let allow_missing_formatter = true;
        let mut stats = Statistics::init();

        let formatters = load_formatters(
            root,
            tree_root,
            allow_missing_formatter,
            &selected_formatters,
            &mut stats,
        )
        .unwrap();

        assert_eq!(formatters.len(), 2);
    }
    #[test]
    #[should_panic]
    fn test_formatter_loading_selected_missing() {
        let tmpdir = utils::tmp_mkdir();

        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        let elm_fmt = tmpdir.path().join("elm-fmt");
        utils::write_binary_file(&black, " ");
        utils::write_binary_file(&nixpkgs_fmt, " ");
        utils::write_binary_file(&elm_fmt, " ");

        let config = format!(
            "
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\"]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]

        [formatter.elm]
        command = {elm_fmt:?}
        options = [\"--yes\"]
        includes = [\"*.elm\"]
        "
        );

        let root = from_string(&config).unwrap();
        let tree_root = tmpdir.path();

        // Selecting a different formatter
        let selected_formatters = Some(vec!["python".into(), "nix".into(), "gofmt".into()]);
        let allow_missing_formatter = false;
        let mut stats = Statistics::init();

        load_formatters(
            root,
            tree_root,
            allow_missing_formatter,
            &selected_formatters,
            &mut stats,
        )
        .unwrap();
    }

    #[test]
    fn test_walker_no_matches() {
        let tmpdir = utils::tmp_mkdir();

        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        let elm_fmt = tmpdir.path().join("elm-fmt");
        utils::write_binary_file(&black, " ");
        utils::write_binary_file(&nixpkgs_fmt, " ");
        utils::write_binary_file(&elm_fmt, " ");

        let config = format!(
            "
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\"]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]

        [formatter.elm]
        command = {elm_fmt:?}
        options = [\"--yes\"]
        includes = [\"*.elm\"]
        "
        );

        let root = from_string(&config).unwrap();
        let tree_root = tmpdir.path();
        let mut stats = Statistics::init();

        let formatters = load_formatters(root, tree_root, false, &None, &mut stats).unwrap();

        let walker = build_walker(vec![tree_root.to_path_buf()], false);
        let _matches = collect_matches_from_walker(walker, &formatters, &mut stats);

        assert_eq!(stats.traversed_files, 3);
        assert_eq!(stats.matched_files, 0);
    }
    #[test]
    fn test_walker_some_matches() {
        let tmpdir = utils::tmp_mkdir();

        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        let elm_fmt = tmpdir.path().join("elm-fmt");
        utils::write_binary_file(&black, " ");
        utils::write_binary_file(&nixpkgs_fmt, " ");
        utils::write_binary_file(&elm_fmt, " ");
        let tree_root = tmpdir.path();

        let files = vec!["test", "test1", "test3", ".test4"];

        for file in files {
            utils::write_file(tree_root.join(format!("{file}.py")), " ");
            utils::write_file(tree_root.join(format!("{file}.nix")), " ");
            utils::write_file(tree_root.join(format!("{file}.elm")), " ");
            utils::write_file(tree_root.join(file), " ");
        }

        let config = format!(
            "
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\"]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]

        [formatter.elm]
        command = {elm_fmt:?}
        options = [\"--yes\"]
        includes = [\"*.elm\"]
        "
        );

        let root = from_string(&config).unwrap();
        let mut stats = Statistics::init();

        let formatters = load_formatters(root, tree_root, false, &None, &mut stats).unwrap();

        let walker = build_walker(vec![tree_root.to_path_buf()], false);
        let _matches = collect_matches_from_walker(walker, &formatters, &mut stats);

        assert_eq!(stats.traversed_files, 15);
        assert_eq!(stats.matched_files, 9);
    }
    #[test]
    fn test_walker_some_matches_walk_hidden() {
        let tmpdir = utils::tmp_mkdir();

        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        let elm_fmt = tmpdir.path().join("elm-fmt");
        utils::write_binary_file(&black, " ");
        utils::write_binary_file(&nixpkgs_fmt, " ");
        utils::write_binary_file(&elm_fmt, " ");
        let tree_root = tmpdir.path();

        let files = vec!["test", "test1", "test3", ".test4"];

        for file in files {
            utils::write_file(tree_root.join(format!("{file}.py")), " ");
            utils::write_file(tree_root.join(format!("{file}.nix")), " ");
            utils::write_file(tree_root.join(format!("{file}.elm")), " ");
            utils::write_file(tree_root.join(file), " ");
        }

        let config = format!(
            "
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\"]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]

        [formatter.elm]
        command = {elm_fmt:?}
        options = [\"--yes\"]
        includes = [\"*.elm\"]
        "
        );

        let root = from_string(&config).unwrap();
        let mut stats = Statistics::init();

        let formatters = load_formatters(root, tree_root, false, &None, &mut stats).unwrap();

        let walker = build_walker(vec![tree_root.to_path_buf()], true);
        let _matches = collect_matches_from_walker(walker, &formatters, &mut stats);

        assert_eq!(stats.traversed_files, 19);
        assert_eq!(stats.matched_files, 12);
    }
    #[test]
    fn test_walker_some_matches_specific_include() {
        let tmpdir = utils::tmp_mkdir();

        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        let elm_fmt = tmpdir.path().join("elm-fmt");
        utils::write_binary_file(&black, " ");
        utils::write_binary_file(&nixpkgs_fmt, " ");
        utils::write_binary_file(&elm_fmt, " ");
        let tree_root = tmpdir.path();

        let files = vec!["test", "test1", "test3", ".test4"];

        for file in files {
            utils::write_file(tree_root.join(format!("{file}.py")), " ");
            utils::write_file(tree_root.join(format!("{file}.nix")), " ");
            utils::write_file(tree_root.join(format!("{file}.elm")), " ");
            utils::write_file(tree_root.join(file), " ");
        }

        let config = format!(
            // The hidden file is not being matched.
            "
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\", \"test\", \".test4\"]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]

        [formatter.elm]
        command = {elm_fmt:?}
        options = [\"--yes\"]
        includes = [\"*.elm\", \"test3\"]
        "
        );

        let root = from_string(&config).unwrap();
        let mut stats = Statistics::init();

        let formatters = load_formatters(root, tree_root, false, &None, &mut stats).unwrap();

        let walker = build_walker(vec![tree_root.to_path_buf()], false);
        let _matches = collect_matches_from_walker(walker, &formatters, &mut stats);

        assert_eq!(stats.traversed_files, 15);
        assert_eq!(stats.matched_files, 11);
    }
    #[test]
    fn test_walker_some_matches_local_exclude() {
        let tmpdir = utils::tmp_mkdir();

        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        let elm_fmt = tmpdir.path().join("elm-fmt");
        utils::write_binary_file(&black, " ");
        utils::write_binary_file(&nixpkgs_fmt, " ");
        utils::write_binary_file(&elm_fmt, " ");
        let tree_root = tmpdir.path();

        let files = vec!["test", "test1", "test3", ".test4"];

        for file in files {
            utils::write_file(tree_root.join(format!("{file}.py")), " ");
            utils::write_file(tree_root.join(format!("{file}.nix")), " ");
            utils::write_file(tree_root.join(file), " ");
        }

        let config = format!(
            "
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\", \"test\"]
        excludes = [\"test.py\", \"test1.py\"]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]
        excludes = [\"test.nix\"]

        [formatter.elm]
        command = {elm_fmt:?}
        options = [\"--yes\"]
        includes = [\"*.elm\"]
        "
        );

        let root = from_string(&config).unwrap();
        let mut stats = Statistics::init();

        let formatters = load_formatters(root, tree_root, false, &None, &mut stats).unwrap();

        let walker = build_walker(vec![tree_root.to_path_buf()], false);
        let matches = collect_matches_from_walker(walker, &formatters, &mut stats);

        assert_eq!(stats.traversed_files, 12);
        assert_eq!(stats.matched_files, 4);
        let python_matches: Vec<PathBuf> = matches
            .get(&FormatterName::new("python"))
            .unwrap()
            .keys()
            .cloned()
            .collect();
        let elm_matches = matches.get(&FormatterName::new("elm")).is_none();
        let nix_matches: Vec<PathBuf> = matches
            .get(&FormatterName::new("nix"))
            .unwrap()
            .keys()
            .cloned()
            .collect();
        let expected_python_matches: Vec<PathBuf> = ["test", "test3.py"]
            .iter()
            .map(|p| tree_root.join(p))
            .collect();
        let expected_nix_matches: Vec<PathBuf> = ["test1.nix", "test3.nix"]
            .iter()
            .map(|p| tree_root.join(p))
            .collect();
        assert_eq!(python_matches, expected_python_matches);
        assert_eq!(nix_matches, expected_nix_matches);
        assert!(elm_matches);
    }
    #[test]
    fn test_walker_some_matches_global_exclude() {
        let tmpdir = utils::tmp_mkdir();

        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        let elm_fmt = tmpdir.path().join("elm-fmt");
        utils::write_binary_file(&black, " ");
        utils::write_binary_file(&nixpkgs_fmt, " ");
        utils::write_binary_file(&elm_fmt, " ");
        let tree_root = tmpdir.path();

        let files = vec![
            "test",
            "test1",
            "test3",
            ".test4",
            "not-a-match",
            "still-not-a-match",
        ];

        for file in files {
            utils::write_file(tree_root.join(format!("{file}.py")), " ");
            utils::write_file(tree_root.join(format!("{file}.nix")), " ");
            utils::write_file(tree_root.join(file), " ");
        }

        let config = format!(
            "
        [global]
        excludes = [\"*not*\"]
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\", \"test\"]
        excludes = [\"test.py\", \"test1.py\"]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]
        excludes = [\"test.nix\"]

        [formatter.elm]
        command = {elm_fmt:?}
        options = [\"--yes\"]
        includes = [\"*.elm\"]
        "
        );

        let root = from_string(&config).unwrap();
        let mut stats = Statistics::init();

        let formatters = load_formatters(root, tree_root, false, &None, &mut stats).unwrap();

        let walker = build_walker(vec![tree_root.to_path_buf()], false);
        let matches = collect_matches_from_walker(walker, &formatters, &mut stats);

        assert_eq!(stats.traversed_files, 18);
        assert_eq!(stats.matched_files, 4);
        let python_matches: Vec<PathBuf> = matches
            .get(&FormatterName::new("python"))
            .unwrap()
            .keys()
            .cloned()
            .collect();
        let elm_matches = matches.get(&FormatterName::new("elm")).is_none();
        let nix_matches: Vec<PathBuf> = matches
            .get(&FormatterName::new("nix"))
            .unwrap()
            .keys()
            .cloned()
            .collect();
        let expected_python_matches: Vec<PathBuf> = ["test", "test3.py"]
            .iter()
            .map(|p| tree_root.join(p))
            .collect();
        let expected_nix_matches: Vec<PathBuf> = ["test1.nix", "test3.nix"]
            .iter()
            .map(|p| tree_root.join(p))
            .collect();
        assert_eq!(python_matches, expected_python_matches);
        assert_eq!(nix_matches, expected_nix_matches);
        assert!(elm_matches);
    }
    #[test]
    fn test_walker_some_matches_gitignore() {
        let tmpdir = utils::tmp_mkdir();

        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        utils::write_binary_file(&black, " ");
        utils::write_binary_file(&nixpkgs_fmt, " ");
        let tree_root = tmpdir.path();

        utils::Git::new(tmpdir.path().to_path_buf())
            .git_ignore("test1.nix\nresult")
            .create();

        let files = vec!["test", "test1", ".test4"];

        for file in files {
            utils::write_file(tree_root.join(format!("{file}.py")), " ");
            utils::write_file(tree_root.join(format!("{file}.nix")), " ");
            utils::write_file(tree_root.join(file), " ");
        }
        utils::write_file(tree_root.join("result"), " ");

        let config = format!(
            "
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\", \"test\"]
        excludes = [\"test.py\" ]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]
        excludes = [\"test.nix\"]
        "
        );

        let root = from_string(&config).unwrap();
        let mut stats = Statistics::init();

        let formatters = load_formatters(root, tree_root, false, &None, &mut stats).unwrap();

        let walker = build_walker(vec![tree_root.to_path_buf()], false);
        let matches = collect_matches_from_walker(walker, &formatters, &mut stats);

        assert_eq!(stats.traversed_files, 7);
        assert_eq!(stats.matched_files, 2);
        let python_matches: Vec<PathBuf> = matches
            .get(&FormatterName::new("python"))
            .unwrap()
            .keys()
            .cloned()
            .collect();
        let nix_matches: bool = matches.get(&FormatterName::new("nix")).is_none();
        let expected_python_matches: Vec<PathBuf> = ["test", "test1.py"]
            .iter()
            .map(|p| tree_root.join(p))
            .collect();
        assert_eq!(python_matches, expected_python_matches);
        assert!(nix_matches);
    }
    #[test]
    fn test_walker_some_matches_ignore_gitignore() {
        let tmpdir = utils::tmp_mkdir();

        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        utils::write_binary_file(&black, " ");
        utils::write_binary_file(&nixpkgs_fmt, " ");
        let tree_root = tmpdir.path();

        utils::Git::new(tmpdir.path().to_path_buf())
            .git_ignore("test1.nix")
            .ignore("result\ntest1\nignored*")
            .create();

        let files = vec!["test", "test1", ".test4", "ignored"];

        for file in files {
            utils::write_file(tree_root.join(format!("{file}.py")), " ");
            utils::write_file(tree_root.join(format!("{file}.nix")), " ");
            utils::write_file(tree_root.join(file), " ");
        }
        utils::write_file(tree_root.join("result"), " ");

        let config = format!(
            "
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\", \"test\"]
        excludes = [\"test.py\" ]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]
        excludes = [\"test.nix\"]
        "
        );

        let root = from_string(&config).unwrap();
        let mut stats = Statistics::init();

        let formatters = load_formatters(root, tree_root, false, &None, &mut stats).unwrap();

        let walker = build_walker(vec![tree_root.to_path_buf()], false);
        let matches = collect_matches_from_walker(walker, &formatters, &mut stats);

        assert_eq!(stats.traversed_files, 7);
        assert_eq!(stats.matched_files, 3);
        let python_matches: Vec<PathBuf> = matches
            .get(&FormatterName::new("python"))
            .unwrap()
            .keys()
            .cloned()
            .collect();
        let nix_matches: Vec<PathBuf> = matches
            .get(&FormatterName::new("nix"))
            .unwrap()
            .keys()
            .cloned()
            .collect();
        let expected_python_matches: Vec<PathBuf> = ["test", "test1.py"]
            .iter()
            .map(|p| tree_root.join(p))
            .collect();
        let expected_nix_matches: Vec<PathBuf> =
            ["test1.nix"].iter().map(|p| tree_root.join(p)).collect();
        assert_eq!(python_matches, expected_python_matches);
        assert_eq!(nix_matches, expected_nix_matches);
    }
    #[test]
    fn test_walker_some_matches_ignore_gitignore_not_a_git_directory() {
        let tmpdir = utils::tmp_mkdir();

        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        utils::write_binary_file(&black, " ");
        utils::write_binary_file(&nixpkgs_fmt, " ");
        let tree_root = tmpdir.path();

        utils::Git::new(tmpdir.path().to_path_buf())
            .git_ignore("test1.nix")
            .ignore("result\nignored*")
            .write_git(false)
            .create();

        let files = vec!["test", "test1", ".test4", "ignored"];

        for file in files {
            utils::write_file(tree_root.join(format!("{file}.py")), " ");
            utils::write_file(tree_root.join(format!("{file}.nix")), " ");
            utils::write_file(tree_root.join(file), " ");
        }
        utils::write_file(tree_root.join("result"), " ");

        let config = format!(
            "
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\", \"test\"]
        excludes = [\"test.py\" ]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]
        excludes = [\"test.nix\"]
        "
        );

        let root = from_string(&config).unwrap();
        let mut stats = Statistics::init();

        let formatters = load_formatters(root, tree_root, false, &None, &mut stats).unwrap();

        let walker = build_walker(vec![tree_root.to_path_buf()], false);
        let matches = collect_matches_from_walker(walker, &formatters, &mut stats);

        assert_eq!(stats.traversed_files, 8);
        assert_eq!(stats.matched_files, 3);
        let python_matches: Vec<PathBuf> = matches
            .get(&FormatterName::new("python"))
            .unwrap()
            .keys()
            .cloned()
            .collect();
        let nix_matches: Vec<PathBuf> = matches
            .get(&FormatterName::new("nix"))
            .unwrap()
            .keys()
            .cloned()
            .collect();
        let expected_python_matches: Vec<PathBuf> = ["test", "test1.py"]
            .iter()
            .map(|p| tree_root.join(p))
            .collect();
        let expected_nix_matches: Vec<PathBuf> =
            ["test1.nix"].iter().map(|p| tree_root.join(p)).collect();
        assert_eq!(python_matches, expected_python_matches);
        assert_eq!(nix_matches, expected_nix_matches);
    }
    #[test]
    fn test_walker_some_matches_exclude_gitignore() {
        let tmpdir = utils::tmp_mkdir();

        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        utils::write_binary_file(&black, " ");
        utils::write_binary_file(&nixpkgs_fmt, " ");
        let tree_root = tmpdir.path();

        utils::Git::new(tmpdir.path().to_path_buf())
            .git_ignore("test1.nix")
            .exclude("result")
            .create();

        let files = vec!["test", "test1", ".test4"];

        for file in files {
            utils::write_file(tree_root.join(format!("{file}.py")), " ");
            utils::write_file(tree_root.join(format!("{file}.nix")), " ");
            utils::write_file(tree_root.join(file), " ");
        }
        utils::write_file(tree_root.join("result"), " ");

        let config = format!(
            "
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\", \"test\"]
        excludes = [\"test.py\" ]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]
        excludes = [\"test.nix\"]
        "
        );

        let root = from_string(&config).unwrap();
        let mut stats = Statistics::init();

        let formatters = load_formatters(root, tree_root, false, &None, &mut stats).unwrap();

        let walker = build_walker(vec![tree_root.to_path_buf()], false);
        let matches = collect_matches_from_walker(walker, &formatters, &mut stats);

        assert_eq!(stats.traversed_files, 7);
        assert_eq!(stats.matched_files, 2);
        let python_matches: Vec<PathBuf> = matches
            .get(&FormatterName::new("python"))
            .unwrap()
            .keys()
            .cloned()
            .collect();
        let nix_matches: bool = matches.get(&FormatterName::new("nix")).is_none();
        let expected_python_matches: Vec<PathBuf> = ["test", "test1.py"]
            .iter()
            .map(|p| tree_root.join(p))
            .collect();
        assert_eq!(python_matches, expected_python_matches);
        assert!(nix_matches);
    }
    #[test]
    fn test_walker_some_matches_exclude_gitignore_hidden() {
        let tmpdir = utils::tmp_mkdir();

        let black = tmpdir.path().join("black");
        let nixpkgs_fmt = tmpdir.path().join("nixpkgs-fmt");
        utils::write_binary_file(&black, " ");
        utils::write_binary_file(&nixpkgs_fmt, " ");
        let tree_root = tmpdir.path();
        let git_dir = tree_root.join(".git");

        utils::Git::new(tmpdir.path().to_path_buf())
            .git_ignore("test1.nix\n.git/*.nix")
            .exclude("result\n.direnv")
            .create();

        let files = vec!["test", "test1", ".test4"];

        for file in files {
            utils::write_file(tree_root.join(format!("{file}.py")), " ");
            utils::write_file(tree_root.join(format!("{file}.nix")), " ");
            utils::write_file(tree_root.join(format!("{file}.py")), " ");
            utils::write_file(tree_root.join(format!("{file}.nix")), " ");
            utils::write_file(git_dir.join(file), " ");
            utils::write_file(tree_root.join(file), " ");
        }
        utils::write_file(tree_root.join("result"), " ");
        utils::write_file(tree_root.join(".direnv"), " ");

        let config = format!(
            "
        [formatter.python]
        command = {black:?}
        includes = [\"*.py\", \"test\"]
        excludes = [\"test.py\" ]

        [formatter.nix]
        command = {nixpkgs_fmt:?}
        includes = [\"*.nix\"]
        excludes = [\"test.nix\"]
        "
        );

        let root = from_string(&config).unwrap();
        let mut stats = Statistics::init();

        let formatters = load_formatters(root, tree_root, false, &None, &mut stats).unwrap();

        let walker = build_walker(vec![tree_root.to_path_buf()], true);
        let matches = collect_matches_from_walker(walker, &formatters, &mut stats);

        assert_eq!(stats.traversed_files, 15);
        assert_eq!(stats.matched_files, 5);
        let python_matches: Vec<PathBuf> = matches
            .get(&FormatterName::new("python"))
            .unwrap()
            .keys()
            .cloned()
            .collect();
        let nix_matches: Vec<PathBuf> = matches
            .get(&FormatterName::new("nix"))
            .unwrap()
            .keys()
            .cloned()
            .collect();
        let expected_nix_matches: Vec<PathBuf> =
            [".test4.nix"].iter().map(|p| tree_root.join(p)).collect();
        let expected_python_matches: Vec<PathBuf> = [".git/test", ".test4.py", "test", "test1.py"]
            .iter()
            .map(|p| tree_root.join(p))
            .collect();
        assert_eq!(python_matches, expected_python_matches);
        assert_eq!(nix_matches, expected_nix_matches);
    }
}
