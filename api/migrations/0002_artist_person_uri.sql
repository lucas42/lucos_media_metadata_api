-- Add optional eolas Person identity link to the artist table (ADR-0009).
-- NULL means the artist is a group or has not yet been curated;
-- a non-NULL value is the eolas:Person URI that owl:sameAs / eolas:preferredIdentifier
-- should point to in the RDF export.
ALTER TABLE artist ADD COLUMN person_uri TEXT;
