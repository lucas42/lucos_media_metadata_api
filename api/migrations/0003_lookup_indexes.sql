-- Per-track lookups (getAllTagsForTrack, getCollectionsByTrack) otherwise full-scan these tables.
CREATE INDEX IF NOT EXISTS tag_trackid ON tag(trackid);
CREATE INDEX IF NOT EXISTS collection_track_trackid ON collection_track(trackid);
