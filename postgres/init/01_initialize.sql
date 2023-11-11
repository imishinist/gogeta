CREATE DATABASE sampledb;

\c sampledb;

DROP TABLE IF EXISTS sample;
CREATE TABLE sample (
    id integer NOT NULL PRIMARY KEY,
    name char(100) NOT NULL,
    created_date_time timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE sample_id_seq START 1;

-- INSERT INTO sample (id, name) VALUES (nextval('sample_id_seq'), 'sample name');