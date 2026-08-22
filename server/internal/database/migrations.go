package database

import (
	"context"
	"fmt"
)

type migration struct {
	version   int
	name, sql string
}

var migrations = []migration{{1, "foundation", `
CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS server_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
CREATE TABLE IF NOT EXISTS users (id TEXT PRIMARY KEY, username TEXT NOT NULL UNIQUE, password_hash TEXT NOT NULL, role TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, disabled_at TEXT);
CREATE TABLE IF NOT EXISTS devices (id TEXT PRIMARY KEY, user_id TEXT, name TEXT NOT NULL, platform TEXT NOT NULL, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, last_seen_at TEXT, FOREIGN KEY(user_id) REFERENCES users(id));
CREATE TABLE IF NOT EXISTS sessions (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, device_id TEXT, refresh_token_hash TEXT NOT NULL, expires_at TEXT NOT NULL, revoked_at TEXT, created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(user_id) REFERENCES users(id), FOREIGN KEY(device_id) REFERENCES devices(id));
CREATE TABLE IF NOT EXISTS audit_events (id INTEGER PRIMARY KEY AUTOINCREMENT, actor_user_id TEXT, action TEXT NOT NULL, target_type TEXT, target_id TEXT, request_id TEXT, metadata_json TEXT NOT NULL DEFAULT '{}', created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP, FOREIGN KEY(actor_user_id) REFERENCES users(id));
`}, {2, "authentication", `
ALTER TABLE users ADD COLUMN display_name TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN status TEXT NOT NULL DEFAULT 'ACTIVE';
ALTER TABLE devices ADD COLUMN client_name TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN client_version TEXT NOT NULL DEFAULT '';
ALTER TABLE devices ADD COLUMN platform_version TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN token_family_id TEXT NOT NULL DEFAULT '';
ALTER TABLE sessions ADD COLUMN previous_refresh_hash TEXT;
ALTER TABLE sessions ADD COLUMN last_activity_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE sessions ADD COLUMN remote_address TEXT;
CREATE TABLE setup_state (id INTEGER PRIMARY KEY CHECK(id=1), completed_at TEXT);
INSERT INTO setup_state(id, completed_at) VALUES(1, NULL);
CREATE TABLE refresh_token_history (session_id TEXT NOT NULL, token_hash TEXT NOT NULL UNIQUE, rotated_at TEXT NOT NULL, PRIMARY KEY(session_id, token_hash), FOREIGN KEY(session_id) REFERENCES sessions(id));
CREATE INDEX idx_sessions_user_active ON sessions(user_id, revoked_at, expires_at);
CREATE INDEX idx_audit_created ON audit_events(created_at DESC);
`}, {3, "media_library_foundation", `
CREATE TABLE libraries (id TEXT PRIMARY KEY, name TEXT NOT NULL, type TEXT NOT NULL CHECK(type IN ('MOVIES','TV')), enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, updated_at TEXT NOT NULL);
CREATE TABLE library_sources (id TEXT PRIMARY KEY, library_id TEXT NOT NULL, configured_path TEXT NOT NULL, normalized_path TEXT NOT NULL, enabled INTEGER NOT NULL DEFAULT 1, created_at TEXT NOT NULL, last_attempted_scan_at TEXT, last_successful_scan_at TEXT, last_scan_status TEXT NOT NULL DEFAULT 'NEVER', last_scan_error TEXT, UNIQUE(library_id, normalized_path), FOREIGN KEY(library_id) REFERENCES libraries(id) ON DELETE CASCADE);
CREATE TABLE media_files (id TEXT PRIMARY KEY, source_id TEXT NOT NULL, relative_path TEXT NOT NULL, file_name TEXT NOT NULL, base_name TEXT NOT NULL, extension TEXT NOT NULL, parent_path TEXT NOT NULL, size_bytes INTEGER NOT NULL, modified_at_ns INTEGER NOT NULL, availability TEXT NOT NULL CHECK(availability IN ('AVAILABLE','MISSING')), probe_status TEXT NOT NULL, probe_error TEXT, container_format TEXT, duration_seconds REAL, bitrate INTEGER, resolution_class TEXT, hdr_class TEXT, candidate_title TEXT, candidate_year INTEGER, season_number INTEGER, episode_start INTEGER, episode_end INTEGER, created_at TEXT NOT NULL, updated_at TEXT NOT NULL, last_seen_scan_id TEXT, UNIQUE(source_id, relative_path), FOREIGN KEY(source_id) REFERENCES library_sources(id) ON DELETE CASCADE);
CREATE TABLE media_streams (id TEXT PRIMARY KEY, media_file_id TEXT NOT NULL, stream_index INTEGER NOT NULL, stream_type TEXT NOT NULL, codec TEXT, profile TEXT, codec_level INTEGER, width INTEGER, height INTEGER, pixel_format TEXT, bit_depth INTEGER, frame_rate TEXT, scan_type TEXT, bitrate INTEGER, language TEXT, title TEXT, is_default INTEGER NOT NULL DEFAULT 0, is_forced INTEGER NOT NULL DEFAULT 0, channels INTEGER, channel_layout TEXT, sample_rate INTEGER, color_primaries TEXT, color_transfer TEXT, color_space TEXT, color_range TEXT, hearing_impaired INTEGER NOT NULL DEFAULT 0, commentary INTEGER NOT NULL DEFAULT 0, UNIQUE(media_file_id, stream_index), FOREIGN KEY(media_file_id) REFERENCES media_files(id) ON DELETE CASCADE);
CREATE TABLE scan_jobs (id TEXT PRIMARY KEY, library_id TEXT NOT NULL, state TEXT NOT NULL, created_at TEXT NOT NULL, started_at TEXT, completed_at TEXT, directories_visited INTEGER NOT NULL DEFAULT 0, files_discovered INTEGER NOT NULL DEFAULT 0, candidates_found INTEGER NOT NULL DEFAULT 0, files_probed INTEGER NOT NULL DEFAULT 0, files_added INTEGER NOT NULL DEFAULT 0, files_updated INTEGER NOT NULL DEFAULT 0, files_unchanged INTEGER NOT NULL DEFAULT 0, files_missing INTEGER NOT NULL DEFAULT 0, files_failed INTEGER NOT NULL DEFAULT 0, current_relative_path TEXT, error_summary TEXT, FOREIGN KEY(library_id) REFERENCES libraries(id) ON DELETE CASCADE);
CREATE TABLE scan_errors (id INTEGER PRIMARY KEY AUTOINCREMENT, scan_job_id TEXT NOT NULL, source_id TEXT, relative_path TEXT, error_code TEXT NOT NULL, safe_message TEXT NOT NULL, created_at TEXT NOT NULL, FOREIGN KEY(scan_job_id) REFERENCES scan_jobs(id) ON DELETE CASCADE, FOREIGN KEY(source_id) REFERENCES library_sources(id) ON DELETE SET NULL);
CREATE INDEX idx_sources_library ON library_sources(library_id);
CREATE INDEX idx_media_source_availability ON media_files(source_id, availability);
CREATE INDEX idx_media_extension ON media_files(extension);
CREATE INDEX idx_media_resolution ON media_files(resolution_class);
CREATE INDEX idx_stream_codec_type ON media_streams(stream_type, codec);
CREATE INDEX idx_scan_library_created ON scan_jobs(library_id, created_at DESC);
`}, {4, "logical_media_metadata", `
CREATE TABLE movies (id TEXT PRIMARY KEY,title TEXT NOT NULL,original_title TEXT,sort_title TEXT NOT NULL,year INTEGER,release_date TEXT,runtime_minutes INTEGER,overview TEXT,tagline TEXT,content_rating TEXT,status TEXT,original_language TEXT,rating_value REAL,rating_votes INTEGER,primary_provider TEXT,provider_id TEXT,metadata_state TEXT NOT NULL,last_metadata_refresh_at TEXT,manual_override INTEGER NOT NULL DEFAULT 0,orphaned INTEGER NOT NULL DEFAULT 0,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,UNIQUE(primary_provider,provider_id));
CREATE TABLE shows (id TEXT PRIMARY KEY,title TEXT NOT NULL,original_title TEXT,sort_title TEXT NOT NULL,year INTEGER,first_air_date TEXT,status TEXT,overview TEXT,original_language TEXT,rating_value REAL,rating_votes INTEGER,primary_provider TEXT,provider_id TEXT,metadata_state TEXT NOT NULL,last_metadata_refresh_at TEXT,manual_override INTEGER NOT NULL DEFAULT 0,orphaned INTEGER NOT NULL DEFAULT 0,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,UNIQUE(primary_provider,provider_id));
CREATE TABLE seasons (id TEXT PRIMARY KEY,show_id TEXT NOT NULL,season_number INTEGER NOT NULL,title TEXT,overview TEXT,air_date TEXT,provider_id TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,UNIQUE(show_id,season_number),FOREIGN KEY(show_id) REFERENCES shows(id) ON DELETE CASCADE);
CREATE TABLE episodes (id TEXT PRIMARY KEY,season_id TEXT NOT NULL,episode_number INTEGER NOT NULL,title TEXT NOT NULL,overview TEXT,air_date TEXT,runtime_minutes INTEGER,provider_id TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,UNIQUE(season_id,episode_number),FOREIGN KEY(season_id) REFERENCES seasons(id) ON DELETE CASCADE);
CREATE TABLE media_associations (id TEXT PRIMARY KEY,media_file_id TEXT NOT NULL,entity_type TEXT NOT NULL CHECK(entity_type IN ('MOVIE','EPISODE')),entity_id TEXT NOT NULL,association_type TEXT NOT NULL,version_label TEXT,created_at TEXT NOT NULL,UNIQUE(media_file_id,entity_type,entity_id),FOREIGN KEY(media_file_id) REFERENCES media_files(id) ON DELETE CASCADE);
CREATE TABLE external_ids (id TEXT PRIMARY KEY,entity_type TEXT NOT NULL,entity_id TEXT NOT NULL,provider TEXT NOT NULL,external_id TEXT NOT NULL,created_at TEXT NOT NULL,UNIQUE(provider,entity_type,external_id),UNIQUE(entity_type,entity_id,provider));
CREATE TABLE genres (id TEXT PRIMARY KEY,provider TEXT NOT NULL,provider_id TEXT NOT NULL,name TEXT NOT NULL,UNIQUE(provider,provider_id));
CREATE TABLE media_genres (entity_type TEXT NOT NULL,entity_id TEXT NOT NULL,genre_id TEXT NOT NULL,PRIMARY KEY(entity_type,entity_id,genre_id),FOREIGN KEY(genre_id) REFERENCES genres(id) ON DELETE CASCADE);
CREATE TABLE people (id TEXT PRIMARY KEY,name TEXT NOT NULL,primary_provider TEXT,provider_id TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,UNIQUE(primary_provider,provider_id));
CREATE TABLE credits (id TEXT PRIMARY KEY,entity_type TEXT NOT NULL,entity_id TEXT NOT NULL,person_id TEXT NOT NULL,credit_type TEXT NOT NULL,character_name TEXT,display_order INTEGER,UNIQUE(entity_type,entity_id,person_id,credit_type,character_name),FOREIGN KEY(person_id) REFERENCES people(id) ON DELETE CASCADE);
CREATE TABLE production_companies (id TEXT PRIMARY KEY,name TEXT NOT NULL,provider TEXT NOT NULL,provider_id TEXT NOT NULL,UNIQUE(provider,provider_id));
CREATE TABLE media_production_companies (entity_type TEXT NOT NULL,entity_id TEXT NOT NULL,company_id TEXT NOT NULL,PRIMARY KEY(entity_type,entity_id,company_id),FOREIGN KEY(company_id) REFERENCES production_companies(id) ON DELETE CASCADE);
CREATE TABLE artwork (id TEXT PRIMARY KEY,entity_type TEXT NOT NULL,entity_id TEXT NOT NULL,artwork_type TEXT NOT NULL,provider TEXT NOT NULL,provider_path TEXT NOT NULL,language TEXT,width INTEGER,height INTEGER,aspect_ratio REAL,vote_average REAL,selected INTEGER NOT NULL DEFAULT 0,manual_selection INTEGER NOT NULL DEFAULT 0,cached_relative_path TEXT,mime_type TEXT,etag TEXT,created_at TEXT NOT NULL,UNIQUE(entity_type,entity_id,artwork_type,provider,provider_path));
CREATE TABLE metadata_match_attempts (id TEXT PRIMARY KEY,media_file_id TEXT NOT NULL,state TEXT NOT NULL,provider TEXT,selected_provider_id TEXT,score INTEGER,confidence TEXT,explanation_json TEXT NOT NULL,candidates_json TEXT NOT NULL,error_summary TEXT,manual INTEGER NOT NULL DEFAULT 0,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,FOREIGN KEY(media_file_id) REFERENCES media_files(id) ON DELETE CASCADE);
CREATE TABLE metadata_jobs (id TEXT PRIMARY KEY,library_id TEXT,entity_type TEXT,entity_id TEXT,operation TEXT NOT NULL,state TEXT NOT NULL,processed INTEGER NOT NULL DEFAULT 0,matched INTEGER NOT NULL DEFAULT 0,ambiguous INTEGER NOT NULL DEFAULT 0,unmatched INTEGER NOT NULL DEFAULT 0,failed INTEGER NOT NULL DEFAULT 0,created_at TEXT NOT NULL,started_at TEXT,completed_at TEXT,error_summary TEXT,FOREIGN KEY(library_id) REFERENCES libraries(id) ON DELETE SET NULL);
CREATE TABLE metadata_provider_cache (cache_key TEXT PRIMARY KEY,provider TEXT NOT NULL,response_json TEXT NOT NULL,expires_at TEXT NOT NULL,created_at TEXT NOT NULL);
CREATE TABLE logical_library_memberships (library_id TEXT NOT NULL,entity_type TEXT NOT NULL,entity_id TEXT NOT NULL,created_at TEXT NOT NULL,PRIMARY KEY(library_id,entity_type,entity_id),FOREIGN KEY(library_id) REFERENCES libraries(id) ON DELETE CASCADE);
CREATE INDEX idx_movies_sort ON movies(sort_title,year);CREATE INDEX idx_shows_sort ON shows(sort_title,year);CREATE INDEX idx_assoc_entity ON media_associations(entity_type,entity_id);CREATE INDEX idx_match_state ON metadata_match_attempts(state,updated_at);CREATE INDEX idx_artwork_entity ON artwork(entity_type,entity_id,artwork_type,selected);CREATE INDEX idx_metadata_jobs_library ON metadata_jobs(library_id,created_at DESC);
`}, {5, "phase3_completion", `
ALTER TABLE metadata_jobs ADD COLUMN total_files INTEGER NOT NULL DEFAULT 0;
ALTER TABLE metadata_jobs ADD COLUMN already_matched INTEGER NOT NULL DEFAULT 0;
ALTER TABLE movies ADD COLUMN last_metadata_error TEXT;
ALTER TABLE shows ADD COLUMN last_metadata_error TEXT;
CREATE INDEX idx_metadata_jobs_active ON metadata_jobs(library_id,state);
`}, {6, "direct_play", `
CREATE TABLE client_capabilities (id TEXT PRIMARY KEY,user_id TEXT NOT NULL,auth_session_id TEXT NOT NULL,schema_version INTEGER NOT NULL,client_name TEXT NOT NULL,client_version TEXT,platform TEXT,platform_version TEXT,device_model TEXT,containers_json TEXT NOT NULL,video_codecs_json TEXT NOT NULL,audio_codecs_json TEXT NOT NULL,subtitle_formats_json TEXT NOT NULL,max_width INTEGER,max_height INTEGER,hdr_json TEXT NOT NULL,direct_play INTEGER NOT NULL,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,FOREIGN KEY(auth_session_id) REFERENCES sessions(id) ON DELETE CASCADE);
CREATE TABLE playback_sessions (id TEXT PRIMARY KEY,user_id TEXT NOT NULL,auth_session_id TEXT NOT NULL,capability_id TEXT NOT NULL,logical_type TEXT NOT NULL CHECK(logical_type IN ('MOVIE','EPISODE')),logical_id TEXT NOT NULL,media_association_id TEXT,media_file_id TEXT,mode TEXT NOT NULL CHECK(mode IN ('DIRECT_PLAY','UNSUPPORTED')),state TEXT NOT NULL CHECK(state IN ('STARTING','PLAYING','PAUSED','STOPPED','COMPLETED','ERROR')),position_seconds REAL NOT NULL DEFAULT 0,duration_seconds REAL NOT NULL DEFAULT 0,started_at TEXT NOT NULL,last_activity_at TEXT NOT NULL,ended_at TEXT,completion_reason TEXT,error_code TEXT,media_token_hash TEXT,media_token_expires_at TEXT,bytes_served INTEGER NOT NULL DEFAULT 0,FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,FOREIGN KEY(auth_session_id) REFERENCES sessions(id) ON DELETE CASCADE,FOREIGN KEY(capability_id) REFERENCES client_capabilities(id),FOREIGN KEY(media_association_id) REFERENCES media_associations(id) ON DELETE SET NULL,FOREIGN KEY(media_file_id) REFERENCES media_files(id) ON DELETE SET NULL);
CREATE TABLE user_media_progress (user_id TEXT NOT NULL,logical_type TEXT NOT NULL CHECK(logical_type IN ('MOVIE','EPISODE')),logical_id TEXT NOT NULL,position_seconds REAL NOT NULL DEFAULT 0,duration_seconds REAL NOT NULL DEFAULT 0,watched INTEGER NOT NULL DEFAULT 0,last_played_at TEXT NOT NULL,updated_at TEXT NOT NULL,PRIMARY KEY(user_id,logical_type,logical_id),FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
CREATE TABLE playback_history (id INTEGER PRIMARY KEY AUTOINCREMENT,playback_session_id TEXT NOT NULL,user_id TEXT NOT NULL,event_type TEXT NOT NULL,position_seconds REAL NOT NULL DEFAULT 0,created_at TEXT NOT NULL,FOREIGN KEY(playback_session_id) REFERENCES playback_sessions(id) ON DELETE CASCADE,FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
CREATE INDEX idx_playback_active ON playback_sessions(state,last_activity_at);
CREATE INDEX idx_playback_user ON playback_sessions(user_id,started_at DESC);
CREATE INDEX idx_progress_user_recent ON user_media_progress(user_id,watched,last_played_at DESC);
CREATE INDEX idx_capabilities_session ON client_capabilities(auth_session_id,updated_at DESC);
`}, {7, "playback_pipeline", `
CREATE TABLE playback_history_phase4 AS SELECT * FROM playback_history;
DROP TABLE playback_history;
ALTER TABLE playback_sessions RENAME TO playback_sessions_phase4;
CREATE TABLE playback_sessions (id TEXT PRIMARY KEY,user_id TEXT NOT NULL,auth_session_id TEXT NOT NULL,capability_id TEXT NOT NULL,logical_type TEXT NOT NULL CHECK(logical_type IN ('MOVIE','EPISODE')),logical_id TEXT NOT NULL,media_association_id TEXT,media_file_id TEXT,mode TEXT NOT NULL CHECK(mode IN ('DIRECT_PLAY','DIRECT_STREAM','AUDIO_TRANSCODE','UNSUPPORTED')),state TEXT NOT NULL CHECK(state IN ('STARTING','PLAYING','PAUSED','STOPPED','COMPLETED','ERROR')),position_seconds REAL NOT NULL DEFAULT 0,duration_seconds REAL NOT NULL DEFAULT 0,started_at TEXT NOT NULL,last_activity_at TEXT NOT NULL,ended_at TEXT,completion_reason TEXT,error_code TEXT,media_token_hash TEXT,media_token_expires_at TEXT,bytes_served INTEGER NOT NULL DEFAULT 0,selected_audio_track_id TEXT,selected_subtitle_track_id TEXT,pipeline_plan_json TEXT NOT NULL DEFAULT '{}',FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,FOREIGN KEY(auth_session_id) REFERENCES sessions(id) ON DELETE CASCADE,FOREIGN KEY(capability_id) REFERENCES client_capabilities(id),FOREIGN KEY(media_association_id) REFERENCES media_associations(id) ON DELETE SET NULL,FOREIGN KEY(media_file_id) REFERENCES media_files(id) ON DELETE SET NULL);
INSERT INTO playback_sessions(id,user_id,auth_session_id,capability_id,logical_type,logical_id,media_association_id,media_file_id,mode,state,position_seconds,duration_seconds,started_at,last_activity_at,ended_at,completion_reason,error_code,media_token_hash,media_token_expires_at,bytes_served) SELECT id,user_id,auth_session_id,capability_id,logical_type,logical_id,media_association_id,media_file_id,mode,state,position_seconds,duration_seconds,started_at,last_activity_at,ended_at,completion_reason,error_code,media_token_hash,media_token_expires_at,bytes_served FROM playback_sessions_phase4;
DROP TABLE playback_sessions_phase4;
CREATE TABLE playback_history (id INTEGER PRIMARY KEY AUTOINCREMENT,playback_session_id TEXT NOT NULL,user_id TEXT NOT NULL,event_type TEXT NOT NULL,position_seconds REAL NOT NULL DEFAULT 0,created_at TEXT NOT NULL,FOREIGN KEY(playback_session_id) REFERENCES playback_sessions(id) ON DELETE CASCADE,FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
INSERT INTO playback_history SELECT * FROM playback_history_phase4;
DROP TABLE playback_history_phase4;
ALTER TABLE client_capabilities ADD COLUMN max_audio_channels INTEGER NOT NULL DEFAULT 2;
ALTER TABLE client_capabilities ADD COLUMN fragmented_mp4 INTEGER NOT NULL DEFAULT 0;
CREATE TABLE playback_pipeline_instances (id TEXT PRIMARY KEY,playback_session_id TEXT NOT NULL,state TEXT NOT NULL CHECK(state IN ('STARTING','RUNNING','STOPPING','STOPPED','FAILED')),mode TEXT NOT NULL CHECK(mode IN ('DIRECT_STREAM','AUDIO_TRANSCODE')),start_seconds REAL NOT NULL DEFAULT 0,started_at TEXT NOT NULL,running_at TEXT,ended_at TEXT,startup_ms INTEGER,error_code TEXT,safe_diagnostic TEXT,FOREIGN KEY(playback_session_id) REFERENCES playback_sessions(id) ON DELETE CASCADE);
CREATE TABLE sidecar_subtitles (id TEXT PRIMARY KEY,media_file_id TEXT NOT NULL,relative_path TEXT NOT NULL,format TEXT NOT NULL,language TEXT,availability TEXT NOT NULL DEFAULT 'AVAILABLE',size_bytes INTEGER NOT NULL,modified_at_ns INTEGER NOT NULL,UNIQUE(media_file_id,relative_path),FOREIGN KEY(media_file_id) REFERENCES media_files(id) ON DELETE CASCADE);
CREATE INDEX idx_playback_active ON playback_sessions(state,last_activity_at);
CREATE INDEX idx_playback_user ON playback_sessions(user_id,started_at DESC);
CREATE INDEX idx_pipeline_session ON playback_pipeline_instances(playback_session_id,started_at DESC);
`}, {8, "video_transcoding", `
CREATE TABLE playback_history_phase5 AS SELECT * FROM playback_history;
CREATE TABLE playback_pipeline_phase5 AS SELECT * FROM playback_pipeline_instances;
DROP TABLE playback_history;
DROP TABLE playback_pipeline_instances;
ALTER TABLE playback_sessions RENAME TO playback_sessions_phase5;
CREATE TABLE playback_sessions (id TEXT PRIMARY KEY,user_id TEXT NOT NULL,auth_session_id TEXT NOT NULL,capability_id TEXT NOT NULL,logical_type TEXT NOT NULL CHECK(logical_type IN ('MOVIE','EPISODE')),logical_id TEXT NOT NULL,media_association_id TEXT,media_file_id TEXT,mode TEXT NOT NULL CHECK(mode IN ('DIRECT_PLAY','DIRECT_STREAM','AUDIO_TRANSCODE','VIDEO_TRANSCODE','UNSUPPORTED')),state TEXT NOT NULL CHECK(state IN ('STARTING','PLAYING','PAUSED','STOPPED','COMPLETED','ERROR')),position_seconds REAL NOT NULL DEFAULT 0,duration_seconds REAL NOT NULL DEFAULT 0,started_at TEXT NOT NULL,last_activity_at TEXT NOT NULL,ended_at TEXT,completion_reason TEXT,error_code TEXT,media_token_hash TEXT,media_token_expires_at TEXT,bytes_served INTEGER NOT NULL DEFAULT 0,selected_audio_track_id TEXT,selected_subtitle_track_id TEXT,pipeline_plan_json TEXT NOT NULL DEFAULT '{}',FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,FOREIGN KEY(auth_session_id) REFERENCES sessions(id) ON DELETE CASCADE,FOREIGN KEY(capability_id) REFERENCES client_capabilities(id),FOREIGN KEY(media_association_id) REFERENCES media_associations(id) ON DELETE SET NULL,FOREIGN KEY(media_file_id) REFERENCES media_files(id) ON DELETE SET NULL);
INSERT INTO playback_sessions SELECT * FROM playback_sessions_phase5;
DROP TABLE playback_sessions_phase5;
CREATE TABLE playback_history (id INTEGER PRIMARY KEY AUTOINCREMENT,playback_session_id TEXT NOT NULL,user_id TEXT NOT NULL,event_type TEXT NOT NULL,position_seconds REAL NOT NULL DEFAULT 0,created_at TEXT NOT NULL,FOREIGN KEY(playback_session_id) REFERENCES playback_sessions(id) ON DELETE CASCADE,FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
INSERT INTO playback_history SELECT * FROM playback_history_phase5;
DROP TABLE playback_history_phase5;
CREATE TABLE playback_pipeline_instances (id TEXT PRIMARY KEY,playback_session_id TEXT NOT NULL,state TEXT NOT NULL CHECK(state IN ('STARTING','RUNNING','STOPPING','STOPPED','FAILED')),mode TEXT NOT NULL CHECK(mode IN ('DIRECT_STREAM','AUDIO_TRANSCODE','VIDEO_TRANSCODE')),start_seconds REAL NOT NULL DEFAULT 0,started_at TEXT NOT NULL,running_at TEXT,ended_at TEXT,startup_ms INTEGER,error_code TEXT,safe_diagnostic TEXT,FOREIGN KEY(playback_session_id) REFERENCES playback_sessions(id) ON DELETE CASCADE);
INSERT INTO playback_pipeline_instances SELECT * FROM playback_pipeline_phase5;
DROP TABLE playback_pipeline_phase5;
CREATE TABLE transcode_sessions (id TEXT PRIMARY KEY,playback_session_id TEXT NOT NULL UNIQUE,state TEXT NOT NULL CHECK(state IN ('STARTING','RUNNING','STOPPED','FAILED')),backend TEXT NOT NULL,quality_id TEXT NOT NULL,target_width INTEGER,target_height INTEGER,target_bitrate INTEGER,encoded_seconds REAL NOT NULL DEFAULT 0,speed REAL NOT NULL DEFAULT 0,output_bytes INTEGER NOT NULL DEFAULT 0,owned_directory TEXT NOT NULL,started_at TEXT NOT NULL,ended_at TEXT,error_code TEXT,FOREIGN KEY(playback_session_id) REFERENCES playback_sessions(id) ON DELETE CASCADE);
CREATE TABLE transcode_backend_status (backend TEXT PRIMARY KEY,detected INTEGER NOT NULL,available INTEGER NOT NULL,encoder TEXT,decoder TEXT,diagnostic TEXT,checked_at TEXT NOT NULL);
CREATE TABLE user_quality_preferences (user_id TEXT PRIMARY KEY,remote_quality_id TEXT NOT NULL DEFAULT 'auto',remote_bitrate_limit INTEGER NOT NULL DEFAULT 0,updated_at TEXT NOT NULL,FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
CREATE INDEX idx_playback_active ON playback_sessions(state,last_activity_at);
CREATE INDEX idx_playback_user ON playback_sessions(user_id,started_at DESC);
CREATE INDEX idx_pipeline_session ON playback_pipeline_instances(playback_session_id,started_at DESC);
CREATE INDEX idx_transcode_state ON transcode_sessions(state,started_at);
`}, {9, "playback_experience", `
CREATE TABLE user_playback_preferences (user_id TEXT PRIMARY KEY,audio_languages_json TEXT NOT NULL DEFAULT '["en"]',subtitle_languages_json TEXT NOT NULL DEFAULT '["en"]',subtitle_mode TEXT NOT NULL DEFAULT 'WHEN_AUDIO_NOT_PREFERRED' CHECK(subtitle_mode IN ('OFF','ALWAYS','WHEN_AUDIO_NOT_PREFERRED','FORCED_ONLY')),autoplay_next INTEGER NOT NULL DEFAULT 1,local_quality_id TEXT NOT NULL DEFAULT 'auto',remote_quality_id TEXT NOT NULL DEFAULT 'auto',avoid_commentary INTEGER NOT NULL DEFAULT 1,prefer_hearing_impaired INTEGER NOT NULL DEFAULT 0,updated_at TEXT NOT NULL,FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
CREATE TABLE playback_contexts (id TEXT PRIMARY KEY,user_id TEXT NOT NULL,context_type TEXT NOT NULL CHECK(context_type IN ('MOVIE_SINGLE','TV_SERIES','TV_SEASON','CONTINUE_WATCHING')),root_id TEXT,active INTEGER NOT NULL DEFAULT 1,created_at TEXT NOT NULL,updated_at TEXT NOT NULL,ended_at TEXT,FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
ALTER TABLE playback_sessions ADD COLUMN playback_context_id TEXT REFERENCES playback_contexts(id) ON DELETE SET NULL;
CREATE TABLE media_markers (id TEXT PRIMARY KEY,logical_type TEXT NOT NULL CHECK(logical_type IN ('MOVIE','EPISODE')),logical_id TEXT NOT NULL,marker_type TEXT NOT NULL CHECK(marker_type IN ('INTRO','RECAP','CREDITS','POST_CREDITS','CUSTOM')),start_seconds REAL NOT NULL CHECK(start_seconds>=0),end_seconds REAL NOT NULL CHECK(end_seconds>start_seconds),source TEXT NOT NULL CHECK(source IN ('MANUAL','FUTURE_AUTOMATIC')),confidence REAL CHECK(confidence IS NULL OR (confidence>=0 AND confidence<=1)),created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE continue_watching_dismissals (user_id TEXT NOT NULL,logical_type TEXT NOT NULL CHECK(logical_type IN ('MOVIE','EPISODE')),logical_id TEXT NOT NULL,dismissed_at TEXT NOT NULL,PRIMARY KEY(user_id,logical_type,logical_id),FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE);
CREATE INDEX idx_markers_media ON media_markers(logical_type,logical_id,start_seconds);
CREATE INDEX idx_context_user_active ON playback_contexts(user_id,active,updated_at);
`}, {10, "playback_intelligence", `
ALTER TABLE media_markers RENAME TO media_markers_phase7;
CREATE TABLE media_markers (id TEXT PRIMARY KEY,logical_type TEXT NOT NULL CHECK(logical_type IN ('MOVIE','EPISODE')),logical_id TEXT NOT NULL,marker_type TEXT NOT NULL CHECK(marker_type IN ('INTRO','RECAP','CREDITS','POST_CREDITS','CUSTOM')),start_seconds REAL NOT NULL CHECK(start_seconds>=0),end_seconds REAL NOT NULL CHECK(end_seconds>start_seconds),source TEXT NOT NULL CHECK(source IN ('MANUAL','AUTOMATIC_AUDIO','AUTOMATIC_VIDEO','AUTOMATIC_HYBRID','IMPORTED')),confidence REAL CHECK(confidence IS NULL OR (confidence>=0 AND confidence<=1)),active INTEGER NOT NULL DEFAULT 1,review_state TEXT NOT NULL DEFAULT 'ACCEPTED' CHECK(review_state IN ('PENDING','ACCEPTED','REJECTED','SUPERSEDED')),source_identity TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
INSERT INTO media_markers(id,logical_type,logical_id,marker_type,start_seconds,end_seconds,source,confidence,created_at,updated_at) SELECT id,logical_type,logical_id,marker_type,start_seconds,end_seconds,CASE source WHEN 'FUTURE_AUTOMATIC' THEN 'IMPORTED' ELSE source END,confidence,created_at,updated_at FROM media_markers_phase7;
DROP TABLE media_markers_phase7;
CREATE TABLE marker_suppressions (logical_type TEXT NOT NULL,logical_id TEXT NOT NULL,marker_type TEXT NOT NULL,source_identity TEXT NOT NULL,reason TEXT NOT NULL,created_at TEXT NOT NULL,PRIMARY KEY(logical_type,logical_id,marker_type,source_identity));
CREATE TABLE media_fingerprints (media_file_id TEXT NOT NULL,kind TEXT NOT NULL,source_identity TEXT NOT NULL,features_json TEXT NOT NULL,size_bytes INTEGER NOT NULL,created_at TEXT NOT NULL,PRIMARY KEY(media_file_id,kind),FOREIGN KEY(media_file_id) REFERENCES media_files(id) ON DELETE CASCADE);
CREATE TABLE background_jobs (id TEXT PRIMARY KEY,job_type TEXT NOT NULL,target_type TEXT NOT NULL,target_id TEXT NOT NULL,priority INTEGER NOT NULL,state TEXT NOT NULL CHECK(state IN ('QUEUED','RUNNING','COMPLETED','FAILED','CANCELED','INTERRUPTED')),progress REAL NOT NULL DEFAULT 0 CHECK(progress>=0 AND progress<=1),attempts INTEGER NOT NULL DEFAULT 0,request_json TEXT NOT NULL DEFAULT '{}',result_json TEXT NOT NULL DEFAULT '{}',error_summary TEXT,created_at TEXT NOT NULL,started_at TEXT,completed_at TEXT);
CREATE TABLE optimized_media (id TEXT PRIMARY KEY,source_media_file_id TEXT NOT NULL,derived_media_file_id TEXT,logical_type TEXT NOT NULL,logical_id TEXT NOT NULL,profile TEXT NOT NULL,status TEXT NOT NULL CHECK(status IN ('QUEUED','RUNNING','COMPLETED','FAILED','CANCELED')),relative_path TEXT NOT NULL,size_bytes INTEGER,checksum_sha256 TEXT,job_id TEXT NOT NULL,created_at TEXT NOT NULL,completed_at TEXT,FOREIGN KEY(source_media_file_id) REFERENCES media_files(id) ON DELETE RESTRICT,FOREIGN KEY(derived_media_file_id) REFERENCES media_files(id) ON DELETE SET NULL,FOREIGN KEY(job_id) REFERENCES background_jobs(id) ON DELETE RESTRICT);
CREATE TABLE automation_rules (id TEXT PRIMARY KEY,name TEXT NOT NULL,enabled INTEGER NOT NULL DEFAULT 1,trigger_type TEXT NOT NULL,conditions_json TEXT NOT NULL,actions_json TEXT NOT NULL,schedule_json TEXT,timezone TEXT NOT NULL DEFAULT 'UTC',last_execution_at TEXT,created_at TEXT NOT NULL,updated_at TEXT NOT NULL);
CREATE TABLE automation_executions (id TEXT PRIMARY KEY,rule_id TEXT NOT NULL,trigger_type TEXT NOT NULL,event_id TEXT NOT NULL,depth INTEGER NOT NULL DEFAULT 0,state TEXT NOT NULL,matched_count INTEGER NOT NULL DEFAULT 0,action_count INTEGER NOT NULL DEFAULT 0,result_json TEXT NOT NULL DEFAULT '{}',created_at TEXT NOT NULL,completed_at TEXT,UNIQUE(rule_id,event_id),FOREIGN KEY(rule_id) REFERENCES automation_rules(id) ON DELETE CASCADE);
INSERT INTO server_settings(key,value) VALUES('automatic_high_confidence_markers','false') ON CONFLICT(key) DO NOTHING;
CREATE INDEX idx_markers_media ON media_markers(logical_type,logical_id,active,start_seconds);
CREATE INDEX idx_markers_review ON media_markers(review_state,confidence DESC);
CREATE INDEX idx_jobs_state_priority ON background_jobs(state,priority,created_at);
CREATE UNIQUE INDEX idx_jobs_active_dedupe ON background_jobs(job_type,target_type,target_id) WHERE state IN ('QUEUED','RUNNING');
CREATE INDEX idx_optimized_logical ON optimized_media(logical_type,logical_id,status);
CREATE INDEX idx_automation_trigger ON automation_rules(enabled,trigger_type);
`}}

func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP)`); err != nil {
		return err
	}
	for _, item := range migrations {
		var count int
		if err := s.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM schema_migrations WHERE version = ?", item.version).Scan(&count); err != nil {
			return err
		}
		if count != 0 {
			continue
		}
		tx, err := s.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, item.sql); err == nil {
			_, err = tx.ExecContext(ctx, "INSERT INTO schema_migrations(version, name) VALUES(?, ?)", item.version, item.name)
		}
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %d %s: %w", item.version, item.name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", item.version, err)
		}
	}
	return nil
}
