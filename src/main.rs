#![allow(clippy::redundant_closure, clippy::redundant_pattern_matching)]

use log::{error, LevelFilter};
use treefmt::command::{cli_from_args, run_cli};

fn main() {
    if let Err(e) = run() {
        error!("{}", e);
        ::std::process::exit(1);
    }
}

fn run() -> anyhow::Result<()> {
    let cli = cli_from_args()?;

    // Configure the logger
    env_logger::builder()
        .format_timestamp(None)
        .format_target(false)
        .filter_level(match cli.verbosity {
            0 => LevelFilter::Info,
            1 => LevelFilter::Debug,
            _ => LevelFilter::Trace,
        })
        .init();

    run_cli(&cli)?;

    Ok(())
}
