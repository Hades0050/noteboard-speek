//! Тот же контракт, что у Go-сервера, только вызывается через Tauri invoke,
//! а не по HTTP. Фронтенд шлёт бинарный protobuf и имя метода.

use prost::Message;

use crate::app::App;
use crate::db::{Card, CardPatch, Column, ColumnOrder, Note, NotePatch};
use crate::{Error, Result};

pub mod pb {
    include!(concat!(env!("OUT_DIR"), "/noteboard.v1.rs"));
}

pub fn dispatch(app: &App, method: &str, body: &[u8]) -> Result<Vec<u8>> {
    let method = method.trim_start_matches('/');
    match method {
        // ---- notes
        "noteboard.v1.NotesService/ListNotes" => {
            let req: pb::ListNotesRequest = decode(body)?;
            let notes = app.list_notes(&req.query)?;
            encode(pb::ListNotesResponse { notes: notes.iter().map(note).collect() })
        }
        "noteboard.v1.NotesService/GetNote" => {
            let req: pb::GetNoteRequest = decode(body)?;
            encode(pb::GetNoteResponse { note: Some(note(&app.get_note(req.id)?)) })
        }
        "noteboard.v1.NotesService/CreateNote" => {
            let req: pb::CreateNoteRequest = decode(body)?;
            encode(pb::CreateNoteResponse {
                note: Some(note(&app.create_note(&req.title, &req.body)?)),
            })
        }
        "noteboard.v1.NotesService/UpdateNote" => {
            let req: pb::UpdateNoteRequest = decode(body)?;
            let patch = NotePatch {
                title: req.title,
                body: req.body,
                transcript: req.transcript,
                pinned: req.pinned,
            };
            encode(pb::UpdateNoteResponse { note: Some(note(&app.update_note(req.id, &patch)?)) })
        }
        "noteboard.v1.NotesService/DeleteNote" => {
            let req: pb::DeleteNoteRequest = decode(body)?;
            app.delete_note(req.id)?;
            encode(pb::DeleteNoteResponse {})
        }
        "noteboard.v1.NotesService/Retranscribe" => {
            let req: pb::RetranscribeRequest = decode(body)?;
            encode(pb::RetranscribeResponse { note: Some(note(&app.retranscribe(req.id)?)) })
        }

        // ---- board
        "noteboard.v1.BoardService/GetBoard" => {
            let columns = app.board()?;
            encode(pb::GetBoardResponse { columns: columns.iter().map(column).collect() })
        }
        "noteboard.v1.BoardService/CreateColumn" => {
            let req: pb::CreateColumnRequest = decode(body)?;
            encode(pb::CreateColumnResponse { column: Some(column(&app.create_column(&req.title)?)) })
        }
        "noteboard.v1.BoardService/RenameColumn" => {
            let req: pb::RenameColumnRequest = decode(body)?;
            app.rename_column(req.id, &req.title)?;
            encode(pb::RenameColumnResponse {})
        }
        "noteboard.v1.BoardService/DeleteColumn" => {
            let req: pb::DeleteColumnRequest = decode(body)?;
            app.delete_column(req.id)?;
            encode(pb::DeleteColumnResponse {})
        }
        "noteboard.v1.BoardService/CreateCard" => {
            let req: pb::CreateCardRequest = decode(body)?;
            let created = app.create_card(req.column_id, &req.title, &req.body, req.note_id)?;
            encode(pb::CreateCardResponse { card: Some(card(&created)) })
        }
        "noteboard.v1.BoardService/AttachNote" => {
            let req: pb::AttachNoteRequest = decode(body)?;
            let created = app.attach_note(req.note_id, req.column_id)?;
            encode(pb::AttachNoteResponse { card: Some(card(&created)) })
        }
        "noteboard.v1.BoardService/UpdateCard" => {
            let req: pb::UpdateCardRequest = decode(body)?;
            let patch = CardPatch { title: req.title, body: req.body, note_id: req.note_id };
            app.update_card(req.id, &patch)?;
            encode(pb::UpdateCardResponse {})
        }
        "noteboard.v1.BoardService/DeleteCard" => {
            let req: pb::DeleteCardRequest = decode(body)?;
            app.delete_card(req.id)?;
            encode(pb::DeleteCardResponse {})
        }
        "noteboard.v1.BoardService/Reorder" => {
            let req: pb::ReorderRequest = decode(body)?;
            let orders: Vec<ColumnOrder> = req
                .columns
                .into_iter()
                .map(|c| ColumnOrder { column_id: c.column_id, card_ids: c.card_ids })
                .collect();
            app.reorder(&orders)?;
            encode(pb::ReorderResponse {})
        }

        other => Err(Error::Invalid(format!("неизвестный метод {other}"))),
    }
}

fn decode<T: Message + Default>(body: &[u8]) -> Result<T> {
    T::decode(body).map_err(|e| Error::Invalid(e.to_string()))
}

fn encode<T: Message>(msg: T) -> Result<Vec<u8>> {
    Ok(msg.encode_to_vec())
}

fn note(n: &Note) -> pb::Note {
    pb::Note {
        id: n.id,
        title: n.title.clone(),
        body: n.body.clone(),
        audio_path: n.audio_path.clone(),
        transcript: n.transcript.clone(),
        transcribe_status: n.status.clone(),
        transcribe_error: n.error.clone(),
        pinned: n.pinned,
        created_at: n.created_at.clone(),
        updated_at: n.updated_at.clone(),
    }
}

fn card(c: &Card) -> pb::Card {
    pb::Card {
        id: c.id,
        column_id: c.column_id,
        title: c.title.clone(),
        body: c.body.clone(),
        position: c.position,
        note_id: c.note_id,
        note_title: c.note_title.clone(),
        note_status: c.note_status.clone(),
        created_at: c.created_at.clone(),
        updated_at: c.updated_at.clone(),
    }
}

fn column(c: &Column) -> pb::Column {
    pb::Column {
        id: c.id,
        title: c.title.clone(),
        position: c.position,
        cards: c.cards.iter().map(card).collect(),
    }
}
