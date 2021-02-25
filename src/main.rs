#![allow(clippy::redundant_closure, clippy::redundant_pattern_matching)]

use treefmt::command::{cli_from_args, run_cli};
use treefmt::CLOG;

fn main() {
    if let Err(e) = run() {
        CLOG.error(&format!("{}", e));
        ::std::process::exit(1);
    }
}

fn run() -> anyhow::Result<()> {
    let cli = cli_from_args()?;

    CLOG.set_log_level(cli.log_level);

    if cli.quiet {
        CLOG.set_quiet(true);
    }

    run_cli(&cli)?;

    Ok(())
}
