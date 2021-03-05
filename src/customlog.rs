//! Fancy custom log functionality.
#![allow(missing_docs)]

use console::style;
use console::Emoji;
use log::{Level, Metadata, Record};

pub static FOLDER: Emoji = Emoji("\u{1f4c2}", "");
pub static WARN: Emoji = Emoji("\u{26a0}\u{fe0f}", ":-)");
pub static ERROR: Emoji = Emoji("\u{26d4}", "");
pub static INFO: Emoji = Emoji("\u{2139}\u{fe0f}", "");
pub static DEBUG: Emoji = Emoji("\u{1f41b}", "");
pub static TRACE: Emoji = Emoji("\u{1f50e}", "");

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
            Level::Trace => eprintln!(
                "{} {}: {}",
                TRACE,
                style("[DEBUG]").bold().dim(),
                record.args()
            ),
            Level::Debug => eprintln!(
                "{} {}: {}",
                DEBUG,
                style("[DEBUG]").bold().dim(),
                record.args()
            ),
            Level::Info => eprintln!(
                "{} {}: {}",
                INFO,
                style("[INFO]").bold().dim(),
                record.args()
            ),
            Level::Warn => eprintln!(
                "{} {}: {}",
                WARN,
                style("[WARN]").bold().dim(),
                record.args()
            ),
            Level::Error => eprintln!(
                "{} {}: {}",
                ERROR,
                style("[ERR]").bold().dim(),
                record.args()
            ),
        }
    }
    // ignore, stderr is already flushed by default
    fn flush(&self) {}
}
