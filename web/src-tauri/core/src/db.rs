use std::path::Path;
use std::time::Duration;

use rusqlite::{functions::FunctionFlags, params, Connection, OptionalExtension, Row};

use crate::{now, Error, Result};

pub const POSITION_STEP: f64 = 1000.0;

#[derive(Debug, Clone, Default)]
pub struct Note {
    pub id: i64,
    pub title: String,
    pub body: String,
    pub audio_path: String,
    pub transcript: String,
    pub status: String,
    pub error: String,
    pub pinned: bool,
    pub created_at: String,
    pub updated_at: String,
}

#[derive(Debug, Clone, Default)]
pub struct NotePatch {
    pub title: Option<String>,
    pub body: Option<String>,
    pub transcript: Option<String>,
    pub pinned: Option<bool>,
}

impl NotePatch {
    fn empty(&self) -> bool {
        self.title.is_none() && self.body.is_none() && self.transcript.is_none() && self.pinned.is_none()
    }
}

#[derive(Debug, Clone, Default)]
pub struct Card {
    pub id: i64,
    pub column_id: i64,
    pub title: String,
    pub body: String,
    pub position: f64,
    pub note_id: Option<i64>,
    pub note_title: String,
    pub note_status: String,
    pub created_at: String,
    pub updated_at: String,
}

#[derive(Debug, Clone, Default)]
pub struct CardPatch {
    pub title: Option<String>,
    pub body: Option<String>,
    pub note_id: Option<i64>,
}

impl CardPatch {
    fn empty(&self) -> bool {
        self.title.is_none() && self.body.is_none() && self.note_id.is_none()
    }
}

#[derive(Debug, Clone, Default)]
pub struct Column {
    pub id: i64,
    pub title: String,
    pub position: f64,
    pub cards: Vec<Card>,
}

#[derive(Debug, Clone)]
pub struct ColumnOrder {
    pub column_id: i64,
    pub card_ids: Vec<i64>,
}

pub type ColumnOrderList = Vec<ColumnOrder>;

const NOTE_COLUMNS: &str = "id, title, body, audio_path, transcript, transcribe_status,
    transcribe_error, pinned, created_at, updated_at";

pub struct Db {
    conn: Connection,
}

impl Db {
    pub fn open(path: &Path) -> Result<Self> {
        let conn = Connection::open(path)?;
        Self::prepare(conn)
    }

    pub fn open_memory() -> Result<Self> {
        let conn = Connection::open_in_memory()?;
        Self::prepare(conn)
    }

    fn prepare(conn: Connection) -> Result<Self> {
        conn.busy_timeout(Duration::from_secs(5))?;
        let _: String = conn.pragma_update_and_check(None, "journal_mode", "WAL", |r| r.get(0))?;
        conn.pragma_update(None, "foreign_keys", "ON")?;
        // SQLite знает регистр только для ASCII, поэтому поиск по русским
        // заметкам без своей функции находит лишь точные совпадения.
        conn.create_scalar_function(
            "ulower",
            1,
            FunctionFlags::SQLITE_UTF8 | FunctionFlags::SQLITE_DETERMINISTIC,
            |ctx| {
                let s: String = ctx.get(0)?;
                Ok(s.to_lowercase())
            },
        )?;
        let db = Self { conn };
        db.migrate()?;
        Ok(db)
    }

    fn migrate(&self) -> Result<()> {
        let version: i64 = self.conn.query_row("PRAGMA user_version", [], |r| r.get(0))?;
        if version >= 1 {
            return Ok(());
        }
        self.conn.execute_batch(
            r#"
            CREATE TABLE IF NOT EXISTS notes (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              title TEXT NOT NULL DEFAULT '',
              body TEXT NOT NULL DEFAULT '',
              audio_path TEXT NOT NULL DEFAULT '',
              transcript TEXT NOT NULL DEFAULT '',
              transcribe_status TEXT NOT NULL DEFAULT 'none',
              transcribe_error TEXT NOT NULL DEFAULT '',
              pinned INTEGER NOT NULL DEFAULT 0,
              created_at TEXT NOT NULL,
              updated_at TEXT NOT NULL
            );
            CREATE INDEX IF NOT EXISTS notes_status ON notes(transcribe_status);

            CREATE TABLE IF NOT EXISTS columns (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              title TEXT NOT NULL,
              position REAL NOT NULL
            );

            CREATE TABLE IF NOT EXISTS cards (
              id INTEGER PRIMARY KEY AUTOINCREMENT,
              column_id INTEGER NOT NULL REFERENCES columns(id) ON DELETE CASCADE,
              title TEXT NOT NULL DEFAULT '',
              body TEXT NOT NULL DEFAULT '',
              position REAL NOT NULL,
              note_id INTEGER REFERENCES notes(id) ON DELETE SET NULL,
              created_at TEXT NOT NULL,
              updated_at TEXT NOT NULL
            );
            CREATE INDEX IF NOT EXISTS cards_column ON cards(column_id, position);
            CREATE INDEX IF NOT EXISTS cards_note ON cards(note_id);

            INSERT INTO columns (title, position)
            SELECT 'Идеи', 1000 WHERE NOT EXISTS (SELECT 1 FROM columns);
            INSERT INTO columns (title, position)
            SELECT 'В работе', 2000 WHERE NOT EXISTS (SELECT 1 FROM columns WHERE position >= 2000);
            INSERT INTO columns (title, position)
            SELECT 'Готово', 3000 WHERE NOT EXISTS (SELECT 1 FROM columns WHERE position >= 3000);

            PRAGMA user_version = 1;
            "#,
        )?;
        Ok(())
    }

    // --- notes ---

    pub fn list_notes(&self, q: &str) -> Result<Vec<Note>> {
        let q = q.trim();
        let mut sql = format!("SELECT {NOTE_COLUMNS} FROM notes");
        if !q.is_empty() {
            sql.push_str(
                " WHERE ulower(title) LIKE ?1 OR ulower(body) LIKE ?1 OR ulower(transcript) LIKE ?1",
            );
        }
        sql.push_str(" ORDER BY pinned DESC, updated_at DESC");

        let mut stmt = self.conn.prepare(&sql)?;
        let rows = if q.is_empty() {
            stmt.query_map([], scan_note)?.collect::<rusqlite::Result<Vec<_>>>()?
        } else {
            let like = format!("%{}%", q.to_lowercase());
            stmt.query_map([like], scan_note)?.collect::<rusqlite::Result<Vec<_>>>()?
        };
        Ok(rows)
    }

    pub fn get_note(&self, id: i64) -> Result<Note> {
        self.conn
            .query_row(&format!("SELECT {NOTE_COLUMNS} FROM notes WHERE id = ?"), [id], scan_note)
            .map_err(Error::from)
    }

    pub fn create_note(&self, title: &str, body: &str) -> Result<Note> {
        let ts = now();
        self.conn.execute(
            "INSERT INTO notes(title, body, created_at, updated_at) VALUES(?,?,?,?)",
            params![title, body, ts, ts],
        )?;
        self.get_note(self.conn.last_insert_rowid())
    }

    pub fn update_note(&self, id: i64, p: &NotePatch) -> Result<Note> {
        if p.empty() {
            return self.get_note(id);
        }
        let mut sets: Vec<&str> = Vec::new();
        let mut args: Vec<Box<dyn rusqlite::ToSql>> = Vec::new();
        if let Some(v) = &p.title {
            sets.push("title = ?");
            args.push(Box::new(v.clone()));
        }
        if let Some(v) = &p.body {
            sets.push("body = ?");
            args.push(Box::new(v.clone()));
        }
        if let Some(v) = &p.transcript {
            sets.push("transcript = ?");
            args.push(Box::new(v.clone()));
        }
        if let Some(v) = p.pinned {
            sets.push("pinned = ?");
            args.push(Box::new(v));
        }
        sets.push("updated_at = ?");
        args.push(Box::new(now()));
        args.push(Box::new(id));

        let sql = format!("UPDATE notes SET {} WHERE id = ?", sets.join(", "));
        let n = self.conn.execute(&sql, rusqlite::params_from_iter(args.iter().map(|a| a.as_ref())))?;
        if n == 0 {
            return Err(Error::NotFound);
        }
        self.get_note(id)
    }

    /// Удаляет заметку вместе с её карточками: карточка без заметки — пустая строка.
    pub fn delete_note(&mut self, id: i64) -> Result<String> {
        let audio = self.get_note(id)?.audio_path;
        let tx = self.conn.transaction()?;
        tx.execute("DELETE FROM cards WHERE note_id = ?", [id])?;
        let n = tx.execute("DELETE FROM notes WHERE id = ?", [id])?;
        if n == 0 {
            return Err(Error::NotFound);
        }
        tx.commit()?;
        Ok(audio)
    }

    pub fn attach_audio(&self, id: i64, path: &str) -> Result<Note> {
        let n = self.conn.execute(
            "UPDATE notes SET audio_path = ?, transcribe_status = 'pending',
             transcribe_error = '', updated_at = ? WHERE id = ?",
            params![path, now(), id],
        )?;
        if n == 0 {
            return Err(Error::NotFound);
        }
        self.get_note(id)
    }

    // --- очередь расшифровки ---

    pub fn claim(&self) -> Result<Option<Note>> {
        let note: Option<Note> = self
            .conn
            .query_row(
                &format!(
                    "SELECT {NOTE_COLUMNS} FROM notes WHERE transcribe_status = 'pending' ORDER BY id LIMIT 1"
                ),
                [],
                scan_note,
            )
            .optional()?;
        let Some(mut note) = note else { return Ok(None) };
        self.conn.execute(
            "UPDATE notes SET transcribe_status = 'running' WHERE id = ?",
            [note.id],
        )?;
        note.status = "running".into();
        Ok(Some(note))
    }

    pub fn finish(&self, id: i64, text: &str) -> Result<()> {
        self.conn.execute(
            "UPDATE notes SET transcribe_status = 'done', transcribe_error = '',
             transcript = ?, updated_at = ? WHERE id = ?",
            params![text, now(), id],
        )?;
        Ok(())
    }

    pub fn fail(&self, id: i64, message: &str) -> Result<()> {
        self.conn.execute(
            "UPDATE notes SET transcribe_status = 'error', transcribe_error = ?,
             updated_at = ? WHERE id = ?",
            params![message, now(), id],
        )?;
        Ok(())
    }

    /// Возвращает в очередь заметки, застрявшие на падении процесса.
    pub fn reset_running(&self) -> Result<()> {
        self.conn.execute(
            "UPDATE notes SET transcribe_status = 'pending' WHERE transcribe_status = 'running'",
            [],
        )?;
        Ok(())
    }

    // --- доска ---

    pub fn board(&self) -> Result<Vec<Column>> {
        let mut stmt = self
            .conn
            .prepare("SELECT id, title, position FROM columns ORDER BY position")?;
        let mut cols: Vec<Column> = stmt
            .query_map([], |r| {
                Ok(Column {
                    id: r.get(0)?,
                    title: r.get(1)?,
                    position: r.get(2)?,
                    cards: Vec::new(),
                })
            })?
            .collect::<rusqlite::Result<Vec<_>>>()?;

        let mut stmt = self.conn.prepare(
            "SELECT c.id, c.column_id, c.title, c.body, c.position, c.note_id,
             COALESCE(n.title, ''), COALESCE(n.transcribe_status, ''), c.created_at, c.updated_at
             FROM cards c LEFT JOIN notes n ON n.id = c.note_id
             ORDER BY c.position",
        )?;
        let cards = stmt
            .query_map([], |r| {
                Ok(Card {
                    id: r.get(0)?,
                    column_id: r.get(1)?,
                    title: r.get(2)?,
                    body: r.get(3)?,
                    position: r.get(4)?,
                    note_id: r.get(5)?,
                    note_title: r.get(6)?,
                    note_status: r.get(7)?,
                    created_at: r.get(8)?,
                    updated_at: r.get(9)?,
                })
            })?
            .collect::<rusqlite::Result<Vec<_>>>()?;

        for card in cards {
            if let Some(col) = cols.iter_mut().find(|c| c.id == card.column_id) {
                col.cards.push(card);
            }
        }
        Ok(cols)
    }

    pub fn create_column(&self, title: &str) -> Result<Column> {
        let pos = self.next_position("SELECT MAX(position) FROM columns", &[])?;
        self.conn.execute(
            "INSERT INTO columns(title, position) VALUES(?, ?)",
            params![title, pos],
        )?;
        Ok(Column {
            id: self.conn.last_insert_rowid(),
            title: title.into(),
            position: pos,
            cards: Vec::new(),
        })
    }

    pub fn rename_column(&self, id: i64, title: &str) -> Result<()> {
        let n = self
            .conn
            .execute("UPDATE columns SET title = ? WHERE id = ?", params![title, id])?;
        if n == 0 {
            return Err(Error::NotFound);
        }
        Ok(())
    }

    pub fn delete_column(&self, id: i64) -> Result<()> {
        let n = self.conn.execute("DELETE FROM columns WHERE id = ?", [id])?;
        if n == 0 {
            return Err(Error::NotFound);
        }
        Ok(())
    }

    pub fn create_card(&self, column_id: i64, title: &str, body: &str, note_id: Option<i64>) -> Result<Card> {
        let pos = self.next_position(
            "SELECT MAX(position) FROM cards WHERE column_id = ?",
            &[&column_id],
        )?;
        let ts = now();
        self.conn.execute(
            "INSERT INTO cards(column_id, title, body, position, note_id, created_at, updated_at)
             VALUES(?,?,?,?,?,?,?)",
            params![column_id, title, body, pos, note_id, ts, ts],
        )?;
        Ok(Card {
            id: self.conn.last_insert_rowid(),
            column_id,
            title: title.into(),
            body: body.into(),
            position: pos,
            note_id,
            created_at: ts.clone(),
            updated_at: ts,
            ..Default::default()
        })
    }

    pub fn update_card(&self, id: i64, p: &CardPatch) -> Result<()> {
        if p.empty() {
            return Ok(());
        }
        let mut sets: Vec<&str> = Vec::new();
        let mut args: Vec<Box<dyn rusqlite::ToSql>> = Vec::new();
        if let Some(v) = &p.title {
            sets.push("title = ?");
            args.push(Box::new(v.clone()));
        }
        if let Some(v) = &p.body {
            sets.push("body = ?");
            args.push(Box::new(v.clone()));
        }
        if let Some(v) = p.note_id {
            sets.push("note_id = ?");
            args.push(Box::new(v));
        }
        sets.push("updated_at = ?");
        args.push(Box::new(now()));
        args.push(Box::new(id));

        let sql = format!("UPDATE cards SET {} WHERE id = ?", sets.join(", "));
        let n = self.conn.execute(&sql, rusqlite::params_from_iter(args.iter().map(|a| a.as_ref())))?;
        if n == 0 {
            return Err(Error::NotFound);
        }
        Ok(())
    }

    pub fn delete_card(&self, id: i64) -> Result<()> {
        let n = self.conn.execute("DELETE FROM cards WHERE id = ?", [id])?;
        if n == 0 {
            return Err(Error::NotFound);
        }
        Ok(())
    }

    /// Переписывает раскладку целых колонок одной транзакцией: перетаскивание
    /// не должно оставлять доску в половинчатом состоянии.
    pub fn reorder(&mut self, orders: &[ColumnOrder]) -> Result<()> {
        let ts = now();
        let tx = self.conn.transaction()?;
        {
            let mut stmt =
                tx.prepare("UPDATE cards SET column_id = ?, position = ?, updated_at = ? WHERE id = ?")?;
            for order in orders {
                for (i, id) in order.card_ids.iter().enumerate() {
                    stmt.execute(params![
                        order.column_id,
                        (i as f64 + 1.0) * POSITION_STEP,
                        ts,
                        id
                    ])?;
                }
            }
        }
        tx.commit()?;
        Ok(())
    }

    /// Конец списка: максимум плюс шаг.
    fn next_position(&self, sql: &str, args: &[&dyn rusqlite::ToSql]) -> Result<f64> {
        let max: Option<f64> = self.conn.query_row(sql, args, |r| r.get(0))?;
        Ok(max.unwrap_or(0.0) + POSITION_STEP)
    }
}

fn scan_note(r: &Row<'_>) -> rusqlite::Result<Note> {
    Ok(Note {
        id: r.get(0)?,
        title: r.get(1)?,
        body: r.get(2)?,
        audio_path: r.get(3)?,
        transcript: r.get(4)?,
        status: r.get(5)?,
        error: r.get(6)?,
        pinned: r.get(7)?,
        created_at: r.get(8)?,
        updated_at: r.get(9)?,
    })
}
