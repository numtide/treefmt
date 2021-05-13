//! Fancy custom log functionality.
#![allow(missing_docs)]

use console::style;
use log::{Level, Metadata, Record};

pub static CUSTOM_LOG: CustomLog = CustomLog {};

/// Synchronized log bar and status message printing.
pub struct CustomLog {}

impl log::Log for CustomLog {
    fn enabled(&self, _metadata: &Metadata) -> bool {
        // The log crate already has log::set_max_level to filter out the logs.
        // We don't need more than that.
        true
    }

    fn log(&self, record: &Record) {
        match record.level() {
            Level::Trace => eprintln!("{}: {}", style("[DEBUG]").bold().dim(), record.args()),
            Level::Debug => eprintln!("{}: {}", style("[DEBUG]").bold().dim(), record.args()),
            Level::Info => eprintln!("{}: {}", style("[INFO]").bold().dim(), record.args()),
            Level::Warn => eprintln!("{}: {}", style("[WARN]").bold().dim(), record.args()),
            Level::Error => eprintln!("{}: {}", style("[ERR]").bold().dim(), record.args()),
        }
    }
    // ignore, stderr is already flushed by default
    fn flush(&self) {}
}
