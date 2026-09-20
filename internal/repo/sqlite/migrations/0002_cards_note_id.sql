-- Связь карточка -> заметка. SQLite не умеет ADD COLUMN IF NOT EXISTS,
-- поэтому раннер отдельно терпит ошибку "duplicate column name" на базах,
-- которые появились до таблицы schema_migrations.
ALTER TABLE cards ADD COLUMN note_id INTEGER REFERENCES notes(id) ON DELETE SET NULL;
