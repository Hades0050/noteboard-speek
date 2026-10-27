pub mod app;
pub mod db;
pub mod rpc;
pub mod stt;
pub mod title;

#[cfg(test)]
mod tests;

use std::fmt;

pub use app::App;
pub use db::{Card, CardPatch, Column, ColumnOrder, Db, Note, NotePatch, ColumnOrderList};

#[derive(Debug)]
pub enum Error {
    NotFound,
    Invalid(String),
    Other(String),
}

pub type Result<T> = std::result::Result<T, Error>;

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Error::NotFound => write!(f, "not found"),
            Error::Invalid(m) | Error::Other(m) => write!(f, "{m}"),
        }
    }
}

impl std::error::Error for Error {}

impl From<rusqlite::Error> for Error {
    fn from(e: rusqlite::Error) -> Self {
        match e {
            rusqlite::Error::QueryReturnedNoRows => Error::NotFound,
            other => Error::Other(other.to_string()),
        }
    }
}

impl From<std::io::Error> for Error {
    fn from(e: std::io::Error) -> Self {
        Error::Other(e.to_string())
    }
}

pub fn now() -> String {
    chrono::Utc::now().to_rfc3339_opts(chrono::SecondsFormat::Secs, true)
}
