-- Пустая доска бесполезна: даём три стандартные колонки, но только если их нет.
INSERT INTO columns (title, position)
SELECT 'Идеи', 1000 WHERE NOT EXISTS (SELECT 1 FROM columns);
INSERT INTO columns (title, position)
SELECT 'В работе', 2000 WHERE NOT EXISTS (SELECT 1 FROM columns WHERE position >= 2000);
INSERT INTO columns (title, position)
SELECT 'Готово', 3000 WHERE NOT EXISTS (SELECT 1 FROM columns WHERE position >= 3000);
