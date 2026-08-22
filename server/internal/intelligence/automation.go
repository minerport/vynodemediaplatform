package intelligence

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

var allowedTriggers = map[string]bool{"MEDIA_ADDED": true, "MEDIA_IDENTIFIED": true, "METADATA_REFRESHED": true, "SCAN_COMPLETED": true, "SCHEDULE": true}
var allowedFields = map[string]bool{"logicalType": true, "libraryType": true, "resolution": true, "codec": true, "hdr": true, "year": true, "rating": true, "availability": true}
var allowedActions = map[string]bool{"RUN_MARKER_ANALYSIS": true, "CREATE_OPTIMIZED_VERSION": true, "ADD_TO_COLLECTION": true, "REMOVE_FROM_COLLECTION": true}

type CollectionManager interface {
	AutomationMembership(context.Context, string, string, string, string) error
}

func (s *Service) ConfigureCollections(c CollectionManager) { s.collections = c }

func validateRule(r Rule) error {
	if strings.TrimSpace(r.Name) == "" || !allowedTriggers[r.Trigger] || len(r.Actions) == 0 {
		return ErrValidation
	}
	for _, c := range r.Conditions {
		if !allowedFields[c.Field] || !contains([]string{"EQUALS", "NOT_EQUALS", "GREATER_THAN", "LESS_THAN"}, c.Operator) {
			return ErrValidation
		}
	}
	for _, a := range r.Actions {
		if !allowedActions[a.Type] {
			return ErrValidation
		}
		if a.Type == "CREATE_OPTIMIZED_VERSION" {
			if _, ok := Profiles[a.Profile]; !ok {
				return ErrValidation
			}
		}
		if (a.Type == "ADD_TO_COLLECTION" || a.Type == "REMOVE_FROM_COLLECTION") && a.CollectionID == "" {
			return ErrValidation
		}
	}
	if r.Timezone == "" {
		r.Timezone = "UTC"
	}
	if _, e := time.LoadLocation(r.Timezone); e != nil {
		return ErrValidation
	}
	if r.Trigger == "SCHEDULE" && (r.Schedule == nil || r.Schedule.Hour < 0 || r.Schedule.Hour > 23 || r.Schedule.Minute < 0 || r.Schedule.Minute > 59) {
		return ErrValidation
	}
	return nil
}
func (s *Service) SaveRule(ctx context.Context, r Rule) (Rule, error) {
	if e := validateRule(r); e != nil {
		return r, e
	}
	if r.ID == "" {
		r.ID = ident()
	}
	c, _ := json.Marshal(r.Conditions)
	a, _ := json.Marshal(r.Actions)
	schedule, _ := json.Marshal(r.Schedule)
	now := stamp(s.now())
	_, e := s.db.ExecContext(ctx, `INSERT INTO automation_rules(id,name,enabled,trigger_type,conditions_json,actions_json,schedule_json,timezone,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?) ON CONFLICT(id) DO UPDATE SET name=excluded.name,enabled=excluded.enabled,trigger_type=excluded.trigger_type,conditions_json=excluded.conditions_json,actions_json=excluded.actions_json,schedule_json=excluded.schedule_json,timezone=excluded.timezone,updated_at=excluded.updated_at`, r.ID, r.Name, r.Enabled, r.Trigger, string(c), string(a), string(schedule), r.Timezone, now, now)
	return r, e
}
func (s *Service) Rules(ctx context.Context) ([]Rule, error) {
	rows, e := s.db.QueryContext(ctx, "SELECT id,name,enabled,trigger_type,conditions_json,actions_json,COALESCE(schedule_json,'null'),timezone,COALESCE(last_execution_at,'') FROM automation_rules ORDER BY name")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Rule{}
	for rows.Next() {
		var r Rule
		var c, a, schedule string
		if e = rows.Scan(&r.ID, &r.Name, &r.Enabled, &r.Trigger, &c, &a, &schedule, &r.Timezone, &r.LastExecutionAt); e != nil {
			return nil, e
		}
		_ = json.Unmarshal([]byte(c), &r.Conditions)
		_ = json.Unmarshal([]byte(a), &r.Actions)
		_ = json.Unmarshal([]byte(schedule), &r.Schedule)
		out = append(out, r)
	}
	return out, rows.Err()
}
func (s *Service) DeleteRule(ctx context.Context, id string) error {
	r, e := s.db.ExecContext(ctx, "DELETE FROM automation_rules WHERE id=?", id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

type target struct {
	ID, LogicalType, FileID, Codec, Resolution, HDR, Availability string
	Year                                                          int
	Rating                                                        float64
}

func (s *Service) targets(ctx context.Context) ([]target, error) {
	rows, e := s.db.QueryContext(ctx, `SELECT a.entity_id,a.entity_type,f.id,COALESCE(v.codec,''),COALESCE(f.resolution_class,''),COALESCE(f.hdr_class,''),f.availability,COALESCE(CASE a.entity_type WHEN 'MOVIE' THEN m.year ELSE CAST(substr(e.air_date,1,4) AS INTEGER) END,0),COALESCE(CASE a.entity_type WHEN 'MOVIE' THEN m.rating_value ELSE 0 END,0) FROM media_associations a JOIN media_files f ON f.id=a.media_file_id LEFT JOIN media_streams v ON v.media_file_id=f.id AND v.stream_type='video' LEFT JOIN movies m ON a.entity_type='MOVIE' AND m.id=a.entity_id LEFT JOIN episodes e ON a.entity_type='EPISODE' AND e.id=a.entity_id WHERE a.association_type!='OPTIMIZED'`)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []target{}
	for rows.Next() {
		var t target
		if e = rows.Scan(&t.ID, &t.LogicalType, &t.FileID, &t.Codec, &t.Resolution, &t.HDR, &t.Availability, &t.Year, &t.Rating); e != nil {
			return nil, e
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
func match(t target, cs []Condition) bool {
	for _, c := range cs {
		var actual any
		switch c.Field {
		case "logicalType":
			actual = t.LogicalType
		case "codec":
			actual = t.Codec
		case "resolution":
			actual = t.Resolution
		case "hdr":
			actual = t.HDR
		case "availability":
			actual = t.Availability
		case "year":
			actual = float64(t.Year)
		case "rating":
			actual = t.Rating
		case "libraryType":
			if t.LogicalType == "MOVIE" {
				actual = "MOVIES"
			} else {
				actual = "TV"
			}
		}
		want := fmt.Sprint(c.Value)
		got := fmt.Sprint(actual)
		switch c.Operator {
		case "EQUALS":
			if !strings.EqualFold(got, want) {
				return false
			}
		case "NOT_EQUALS":
			if strings.EqualFold(got, want) {
				return false
			}
		case "GREATER_THAN":
			var n float64
			fmt.Sscan(want, &n)
			var g float64
			fmt.Sscan(got, &g)
			if g <= n {
				return false
			}
		case "LESS_THAN":
			var n float64
			fmt.Sscan(want, &n)
			var g float64
			fmt.Sscan(got, &g)
			if g >= n {
				return false
			}
		}
	}
	return true
}
func (s *Service) DryRun(ctx context.Context, r Rule) (DryRun, error) {
	if e := validateRule(r); e != nil {
		return DryRun{}, e
	}
	ts, e := s.targets(ctx)
	if e != nil {
		return DryRun{}, e
	}
	out := DryRun{Matches: []string{}}
	for _, t := range ts {
		if match(t, r.Conditions) {
			out.Matches = append(out.Matches, t.LogicalType+":"+t.ID)
		}
	}
	return out, nil
}
func (s *Service) Execute(ctx context.Context, ruleID, eventID string, depth int) (DryRun, error) {
	if depth > 3 {
		return DryRun{}, ErrValidation
	}
	rules, e := s.Rules(ctx)
	if e != nil {
		return DryRun{}, e
	}
	var r Rule
	found := false
	for _, x := range rules {
		if x.ID == ruleID && x.Enabled {
			r = x
			found = true
		}
	}
	if !found {
		return DryRun{}, ErrNotFound
	}
	execID := ident()
	res, e := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO automation_executions(id,rule_id,trigger_type,event_id,depth,state,created_at) VALUES(?,?,?,?,?,'RUNNING',?)", execID, r.ID, r.Trigger, eventID, depth, stamp(s.now()))
	if e != nil {
		return DryRun{}, e
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return DryRun{}, nil
	}
	dry, e := s.DryRun(ctx, r)
	if e != nil {
		return dry, e
	}
	ts, _ := s.targets(ctx)
	actions := 0
	for _, t := range ts {
		if !match(t, r.Conditions) {
			continue
		}
		for _, a := range r.Actions {
			switch a.Type {
			case "CREATE_OPTIMIZED_VERSION":
				_, e = s.Optimize(ctx, t.LogicalType, t.ID, t.FileID, a.Profile)
				if e == nil {
					actions++
				}
			case "RUN_MARKER_ANALYSIS":
				_, e = s.Analyze(ctx, t.LogicalType, t.ID)
				if e == nil {
					actions++
				}
			case "ADD_TO_COLLECTION", "REMOVE_FROM_COLLECTION":
				if s.collections != nil && t.LogicalType != "EPISODE" {
					e = s.collections.AutomationMembership(ctx, a.CollectionID, a.Type, t.LogicalType, t.ID)
					if e == nil {
						actions++
					}
				}
			}
		}
	}
	dry.Actions = actions
	b, _ := json.Marshal(dry)
	_, _ = s.db.ExecContext(ctx, "UPDATE automation_executions SET state='COMPLETED',matched_count=?,action_count=?,result_json=?,completed_at=? WHERE id=?", len(dry.Matches), actions, string(b), stamp(s.now()), execID)
	_, _ = s.db.ExecContext(ctx, "UPDATE automation_rules SET last_execution_at=? WHERE id=?", stamp(s.now()), r.ID)
	return dry, nil
}

// RunDue executes each enabled schedule rule at most once per UTC minute. The
// persisted execution key makes repeated scheduler ticks idempotent.
func (s *Service) RunDue(ctx context.Context, now time.Time) error {
	rules, e := s.Rules(ctx)
	if e != nil {
		return e
	}
	minute := now.UTC().Format("2006-01-02T15:04")
	for _, r := range rules {
		loc, _ := time.LoadLocation(r.Timezone)
		local := now.In(loc)
		if r.Enabled && r.Trigger == "SCHEDULE" && r.Schedule != nil && local.Hour() == r.Schedule.Hour && local.Minute() == r.Schedule.Minute {
			_, e = s.Execute(ctx, r.ID, "schedule:"+minute, 0)
			if e != nil && e != sql.ErrNoRows {
				return e
			}
		}
	}
	return nil
}
func (s *Service) HandleEvent(ctx context.Context, trigger, eventID string) {
	if !allowedTriggers[trigger] || eventID == "" {
		return
	}
	rules, e := s.Rules(ctx)
	if e != nil {
		return
	}
	for _, r := range rules {
		if r.Enabled && r.Trigger == trigger {
			_, _ = s.Execute(ctx, r.ID, trigger+":"+eventID, 0)
		}
	}
}
