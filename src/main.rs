#![allow(clippy::redundant_closure, clippy::redundant_pattern_matching)]

use log::error;
use treefmt::command::{cli_from_args, run_cli};
use treefmt::customlog::CUSTOM_LOG;

fn main() {
    // Configure the logger
    log::set_logger(&CUSTOM_LOG).expect("Could not set the logger");
    // The default log level
    log::set_max_level(log::LevelFilter::Info);

    if let Err(e) = run() {
        error!("{}", e);
        ::std::process::exit(1);
    }
}

fn run() -> anyhow::Result<()> {
    let cli = cli_from_args()?;

    if cli.quiet {
        log::set_max_level(log::LevelFilter::Off)
    } else if cli.verbosity > 0 {
        log::set_max_level(log::LevelFilter::Trace)
    }

    run_cli(&cli)?;

    Ok(())
}
