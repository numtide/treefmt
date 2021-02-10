//! Fancy custom log functionality.
#![allow(missing_docs)]

use anyhow;
use console::style;
use console::Emoji;
use std::sync::atomic::{AtomicBool, AtomicU8, Ordering};

pub static FOLDER: Emoji = Emoji("📂", "");
pub static WARN: Emoji = Emoji("⚠️", ":-)");
pub static ERROR: Emoji = Emoji("⛔", "");
pub static INFO: Emoji = Emoji("ℹ️", "");
pub static DEBUG: Emoji = Emoji("🐛", "");

#[repr(u8)]
#[derive(Debug, Clone, Copy)]
/// The log level for prjfmt
// Important! the least verbose must be at
// the top and the most verbose at the bottom
pub enum LogLevel {
    /// Logs only error
    Error,
    /// Logs only warn and error
    Warn,
    /// Logs warn, error and info
    Info,
    /// Logs everything
    Debug,
}

impl std::str::FromStr for LogLevel {
    type Err = anyhow::Error;
    fn from_str(s: &str) -> anyhow::Result<Self> {
        match s {
            "error" => Ok(LogLevel::Error),
            "warn" => Ok(LogLevel::Warn),
            "info" => Ok(LogLevel::Info),
            "debug" => Ok(LogLevel::Debug),
            _ => anyhow::bail!("Unknown log-level: {}", s),
        }
    }
}

/// Synchronized log bar and status message printing.
pub struct CustomLogOutput {
    quiet: AtomicBool,
    log_level: AtomicU8,
}

impl CustomLogOutput {
    /// Returns a new CustomLogOutput
    pub const fn new() -> Self {
        Self {
            quiet: AtomicBool::new(false),
            log_level: AtomicU8::new(LogLevel::Info as u8),
        }
    }

    /// Print the given message.
    fn message(&self, message: &str) {
        eprintln!("{}", message);
    }

    /// Returns whether it should silence stdout or not
    pub fn quiet(&self) -> bool {
        self.quiet.load(Ordering::SeqCst)
    }

    /// Causes it to silence stdout
    pub fn set_quiet(&self, quiet: bool) {
        self.quiet.store(quiet, Ordering::SeqCst);
    }

    /// Returns whether the specified log level is enabled or not
    pub fn is_log_enabled(&self, level: LogLevel) -> bool {
        (level as u8) <= self.log_level.load(Ordering::SeqCst)
    }

    /// Sets the log level for prjfmt
    pub fn set_log_level(&self, log_level: LogLevel) {
        self.log_level.store(log_level as u8, Ordering::SeqCst);
    }

    /// Add debug message.
    pub fn debug(&self, message: &str) {
        if !self.quiet() && self.is_log_enabled(LogLevel::Debug) {
            let debug = format!("{} {}: {}", DEBUG, style("[DEBUG]").bold().dim(), message,);
            self.message(&debug);
        }
    }

    /// Add an informational message.
    pub fn info(&self, message: &str) {
        if !self.quiet() && self.is_log_enabled(LogLevel::Info) {
            let info = format!("{} {}: {}", INFO, style("[INFO]").bold().dim(), message,);
            self.message(&info);
        }
    }

    /// Add a warning message.
    pub fn warn(&self, message: &str) {
        if !self.quiet() && self.is_log_enabled(LogLevel::Warn) {
            let warn = format!("{} {}: {}", WARN, style("[WARN]").bold().dim(), message);
            self.message(&warn);
        }
    }

    /// Add an error message.
    pub fn error(&self, message: &str) {
        if self.is_log_enabled(LogLevel::Error) {
            let err = format!("{} {}: {}", ERROR, style("[ERR]").bold().dim(), message);
            self.message(&err);
        }
    }
}

impl Default for CustomLogOutput {
    fn default() -> Self {
        CustomLogOutput::new()
    }
}
