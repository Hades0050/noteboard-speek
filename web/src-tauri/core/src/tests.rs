use crate::db::{CardPatch, ColumnOrder, Db, NotePatch, Note};
use crate::title::card_title;
use crate::{App, Error};

fn app() -> App {
    App::memory().expect("app")
}

fn note_with(body: &str, transcript: &str, title: &str) -> Note {
    Note {
        title: title.into(),
        body: body.into(),
        transcript: transcript.into(),
        ..Default::default()
    }
}

#[test]
fn card_title_prefers_title_then_body_then_transcript() {
    assert_eq!(card_title(&note_with("тело", "расшифровка", "Заголовок")), "Заголовок");
    assert_eq!(card_title(&note_with("<p>тело</p>", "расшифровка", "")), "тело");
    assert_eq!(card_title(&note_with("", "расшифровка", "")), "расшифровка");
    assert_eq!(card_title(&note_with("", "", "")), "Заметка");
}

#[test]
fn card_title_cuts_to_sixty_chars() {
    let long = "я".repeat(80);
    let got = card_title(&note_with(&long, "", ""));
    assert_eq!(got.chars().count(), 61);
    assert!(got.ends_with('…'));
}

#[test]
fn card_title_strips_tags_and_entities() {
    let got = card_title(&note_with("<p>раз&nbsp;два</p><p>три</p>", "", ""));
    assert_eq!(got, "раз два три");
}

#[test]
fn seeded_board_has_three_columns() {
    let cols = app().board().unwrap();
    assert_eq!(
        cols.iter().map(|c| c.title.as_str()).collect::<Vec<_>>(),
        ["Идеи", "В работе", "Готово"]
    );
    assert_eq!(cols[0].position, 1000.0);
}

#[test]
fn new_card_goes_to_end_of_column() {
    let app = app();
    let col = app.board().unwrap()[0].id;
    let first = app.create_card(col, "раз", "", None).unwrap();
    let second = app.create_card(col, "два", "", None).unwrap();
    assert_eq!(first.position, 1000.0);
    assert_eq!(second.position, 2000.0);
}

#[test]
fn reorder_moves_card_between_columns_and_renumbers() {
    let app = app();
    let cols = app.board().unwrap();
    let (a, b) = (cols[0].id, cols[1].id);
    let c1 = app.create_card(a, "раз", "", None).unwrap();
    let c2 = app.create_card(a, "два", "", None).unwrap();

    app.reorder(&[
        ColumnOrder { column_id: a, card_ids: vec![c2.id] },
        ColumnOrder { column_id: b, card_ids: vec![c1.id] },
    ])
    .unwrap();

    let cols = app.board().unwrap();
    assert_eq!(cols[0].cards.len(), 1);
    assert_eq!(cols[0].cards[0].id, c2.id);
    assert_eq!(cols[0].cards[0].position, 1000.0);
    assert_eq!(cols[1].cards[0].id, c1.id);
    assert_eq!(cols[1].cards[0].column_id, b);
}

#[test]
fn reorder_rejects_empty_input() {
    assert!(matches!(app().reorder(&[]), Err(Error::Invalid(_))));
}

#[test]
fn attach_note_without_column_takes_first() {
    let app = app();
    let note = app.create_note("Заметка", "").unwrap();
    let card = app.attach_note(note.id, 0).unwrap();
    assert_eq!(card.column_id, app.board().unwrap()[0].id);
    assert_eq!(card.title, "Заметка");
    assert_eq!(card.note_id, Some(note.id));
}

#[test]
fn attach_note_without_columns_fails() {
    let app = app();
    for col in app.board().unwrap() {
        app.delete_column(col.id).unwrap();
    }
    let note = app.create_note("Заметка", "").unwrap();
    assert!(matches!(app.attach_note(note.id, 0), Err(Error::Invalid(_))));
}

#[test]
fn deleting_note_removes_its_cards() {
    let app = app();
    let note = app.create_note("Заметка", "").unwrap();
    app.attach_note(note.id, 0).unwrap();
    app.delete_note(note.id).unwrap();
    assert!(app.board().unwrap().iter().all(|c| c.cards.is_empty()));
    assert!(matches!(app.get_note(note.id), Err(Error::NotFound)));
}

#[test]
fn deleting_column_removes_its_cards_only() {
    let app = app();
    let cols = app.board().unwrap();
    let note = app.create_note("Заметка", "").unwrap();
    app.attach_note(note.id, cols[0].id).unwrap();
    app.delete_column(cols[0].id).unwrap();
    assert_eq!(app.board().unwrap().len(), 2);
    assert!(app.get_note(note.id).is_ok());
}

#[test]
fn patch_touches_only_given_fields() {
    let app = app();
    let note = app.create_note("Заголовок", "тело").unwrap();
    let updated = app
        .update_note(note.id, &NotePatch { pinned: Some(true), ..Default::default() })
        .unwrap();
    assert!(updated.pinned);
    assert_eq!(updated.title, "Заголовок");
    assert_eq!(updated.body, "тело");
}

#[test]
fn search_is_case_insensitive_for_cyrillic() {
    let app = app();
    app.create_note("Планёрка", "Обсудили ИнфоКиоск").unwrap();
    assert_eq!(app.list_notes("планёрка").unwrap().len(), 1);
    assert_eq!(app.list_notes("инфокиоск").unwrap().len(), 1);
    assert_eq!(app.list_notes("ПЛАНЁРКА").unwrap().len(), 1);
    assert_eq!(app.list_notes("нет такого").unwrap().len(), 0);
}

#[test]
fn pinned_notes_come_first() {
    let app = app();
    let first = app.create_note("раз", "").unwrap();
    app.create_note("два", "").unwrap();
    app.update_note(first.id, &NotePatch { pinned: Some(true), ..Default::default() })
        .unwrap();
    assert_eq!(app.list_notes("").unwrap()[0].id, first.id);
}

#[test]
fn queue_runs_pending_and_restores_stuck_running() {
    let db = Db::open_memory().unwrap();
    let note = db.create_note("", "").unwrap();
    db.attach_audio(note.id, "/tmp/a.wav").unwrap();
    assert_eq!(db.get_note(note.id).unwrap().status, "pending");

    let claimed = db.claim().unwrap().expect("claim");
    assert_eq!(claimed.status, "running");
    assert!(db.claim().unwrap().is_none());

    // процесс упал на середине
    db.reset_running().unwrap();
    assert_eq!(db.get_note(note.id).unwrap().status, "pending");

    let claimed = db.claim().unwrap().expect("claim");
    db.finish(claimed.id, "текст").unwrap();
    let done = db.get_note(note.id).unwrap();
    assert_eq!(done.status, "done");
    assert_eq!(done.transcript, "текст");
}

#[test]
fn retranscribe_requires_audio() {
    let app = app();
    let note = app.create_note("", "").unwrap();
    assert!(matches!(app.retranscribe(note.id), Err(Error::Invalid(_))));
}

#[test]
fn card_patch_updates_note_link() {
    let app = app();
    let col = app.board().unwrap()[0].id;
    let note = app.create_note("Заметка", "").unwrap();
    let card = app.create_card(col, "карточка", "", None).unwrap();
    app.update_card(card.id, &CardPatch { note_id: Some(note.id), ..Default::default() })
        .unwrap();
    let board = app.board().unwrap();
    let saved = &board[0].cards[0];
    assert_eq!(saved.note_id, Some(note.id));
    assert_eq!(saved.note_title, "Заметка");
}
