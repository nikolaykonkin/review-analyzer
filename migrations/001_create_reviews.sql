-- Отзывы: исходные данные из CSV и результат анализа LLM
CREATE TABLE reviews (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    product_name TEXT        NOT NULL,
    brand_name   TEXT,
    review_text  TEXT        NOT NULL,
    rating       SMALLINT    NOT NULL CHECK (rating BETWEEN 1 AND 5),
    reviewed_at  DATE        NOT NULL,
    status       TEXT        NOT NULL DEFAULT 'pending'
                 CHECK (status IN ('pending', 'processing', 'done', 'failed')),
    sentiment    TEXT        CHECK (sentiment IN ('positive', 'neutral', 'negative')),
    topics       TEXT[]      NOT NULL DEFAULT '{}',
    priority     TEXT        CHECK (priority IN ('high', 'normal')),
    summary      TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT reviews_done_has_result CHECK (
        status <> 'done'
        OR (sentiment IS NOT NULL AND priority IS NOT NULL AND summary IS NOT NULL)
    )
);