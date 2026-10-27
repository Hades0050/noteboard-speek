use std::fs;
use std::path::{Path, PathBuf};
use std::sync::Mutex;

use crate::db::{Card, CardPatch, Column, ColumnOrder, Db, Note, NotePatch};
use crate::title::card_title;
use crate::{Error, Result};

/// Фасад приложения: база под одним замком плюс файлы на диске.
/// Правила здесь — те же, что были в internal/usecase у Go-версии.
pub struct App {
    db: Mutex<Db>,
    pub data_dir: PathBuf,
}

impl App {
    pub fn open(data_dir: PathBuf) -> Result<Self> {
        fs::create_dir_all(data_dir.join("media"))?;
        let db = Db::open(&data_dir.join("noteboard.db"))?;
        db.reset_running()?;
        Ok(Self { db: Mutex::new(db), data_dir })
    }

    #[cfg(test)]
    pub fn memory() -> Result<Self> {
        let dir = std::env::temp_dir().join("noteboard-test");
        fs::create_dir_all(&dir)?;
        Ok(Self { db: Mutex::new(Db::open_memory()?), data_dir: dir })
    }

    fn db(&self) -> std::sync::MutexGuard<'_, Db> {
        self.db.lock().expect("db mutex")
    }

    pub fn media_path(&self, name: &str) -> PathBuf {
        self.data_dir.join("media").join(name)
    }

    // --- заметки ---

    pub fn list_notes(&self, q: &str) -> Result<Vec<Note>> {
        self.db().list_notes(q)
    }

    pub fn get_note(&self, id: i64) -> Result<Note> {
        self.db().get_note(id)
    }

    pub fn create_note(&self, title: &str, body: &str) -> Result<Note> {
        self.db().create_note(title, body)
    }

    pub fn update_note(&self, id: i64, p: &NotePatch) -> Result<Note> {
        self.db().update_note(id, p)
    }

    /// Удаляет заметку вместе с её аудиофайлом: иначе диск растёт молча.
    pub fn delete_note(&self, id: i64) -> Result<()> {
        let audio = self.db().delete_note(id)?;
        if !audio.is_empty() {
            let _ = fs::remove_file(&audio);
        }
        Ok(())
    }

    /// Сохраняет запись и ставит заметку в очередь. Старый файл удаляется
    /// только после успешной записи нового.
    pub fn attach_audio(&self, id: i64, bytes: &[u8], ext: &str) -> Result<Note> {
        let old = self.get_note(id)?;
        let name = format!("note-{id}-{}.{ext}", crate::now().replace(':', "-"));
        let path = self.media_path(&name);
        fs::write(&path, bytes)?;

        let path_string = path.to_string_lossy().into_owned();
        let note = match self.db().attach_audio(id, &path_string) {
            Ok(n) => n,
            Err(e) => {
                let _ = fs::remove_file(&path);
                return Err(e);
            }
        };
        if old.audio_path != path_string && !old.audio_path.is_empty() {
            let _ = fs::remove_file(&old.audio_path);
        }
        Ok(note)
    }

    /// Ставит в очередь уже загруженное аудио — например, после смены модели.
    pub fn retranscribe(&self, id: i64) -> Result<Note> {
        let note = self.get_note(id)?;
        if note.audio_path.is_empty() {
            return Err(Error::Invalid("у заметки нет аудио".into()));
        }
        self.db().attach_audio(id, &note.audio_path)
    }

    // --- очередь ---

    pub fn claim(&self) -> Result<Option<Note>> {
        self.db().claim()
    }

    pub fn finish(&self, id: i64, text: &str) -> Result<()> {
        self.db().finish(id, text)
    }

    pub fn fail(&self, id: i64, message: &str) -> Result<()> {
        self.db().fail(id, message)
    }

    // --- доска ---

    pub fn board(&self) -> Result<Vec<Column>> {
        self.db().board()
    }

    pub fn create_column(&self, title: &str) -> Result<Column> {
        let title = title.trim();
        if title.is_empty() {
            return Err(Error::Invalid("пустой title".into()));
        }
        self.db().create_column(title)
    }

    pub fn rename_column(&self, id: i64, title: &str) -> Result<()> {
        let title = title.trim();
        if title.is_empty() {
            return Err(Error::Invalid("пустой title".into()));
        }
        self.db().rename_column(id, title)
    }

    pub fn delete_column(&self, id: i64) -> Result<()> {
        self.db().delete_column(id)
    }

    /// Если карточка привязана к заметке и заголовок пуст, он берётся из заметки.
    pub fn create_card(&self, column_id: i64, title: &str, body: &str, note_id: Option<i64>) -> Result<Card> {
        if column_id == 0 {
            return Err(Error::Invalid("нужен column_id".into()));
        }
        let mut title = title.to_string();
        let mut note: Option<Note> = None;
        if let Some(nid) = note_id {
            let n = self.get_note(nid)?;
            if title.trim().is_empty() {
                title = card_title(&n);
            }
            note = Some(n);
        }

        let mut card = self.db().create_card(column_id, &title, body, note_id)?;
        if let Some(n) = note {
            card.note_title = n.title;
            card.note_status = n.status;
        }
        Ok(card)
    }

    /// Без column_id заметка попадает в первую колонку — «На доску» должно
    /// работать одним нажатием.
    pub fn attach_note(&self, note_id: i64, column_id: i64) -> Result<Card> {
        let column_id = if column_id == 0 {
            let cols = self.board()?;
            let first = cols.first().ok_or_else(|| Error::Invalid("на доске нет колонок".into()))?;
            first.id
        } else {
            column_id
        };
        self.create_card(column_id, "", "", Some(note_id))
    }

    pub fn update_card(&self, id: i64, p: &CardPatch) -> Result<()> {
        self.db().update_card(id, p)
    }

    pub fn delete_card(&self, id: i64) -> Result<()> {
        self.db().delete_card(id)
    }

    pub fn reorder(&self, orders: &[ColumnOrder]) -> Result<()> {
        if orders.is_empty() {
            return Err(Error::Invalid("пустой columns".into()));
        }
        self.db().reorder(orders)
    }
}

pub fn media_dir(data_dir: &Path) -> PathBuf {
    data_dir.join("media")
}
