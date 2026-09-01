-- +goose Up
INSERT INTO widgets (id, name) VALUES (1, 'wrench'), (2, 'hammer');

-- +goose Down
DELETE FROM widgets;
