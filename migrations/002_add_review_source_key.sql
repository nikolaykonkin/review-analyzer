-- Ключ дедупликации исходных отзывов: повторный импорт не создает дубликаты
-- Допускает NULL (несколько NULL не нарушают UNIQUE); импортер всегда заполняет ключ
ALTER TABLE reviews ADD COLUMN source_key TEXT UNIQUE;