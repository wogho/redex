package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	actool "github.com/Autumn-27/norma/tool"

	"github.com/wogho/redex/db"
)

// Agent bridges: expose OSINT, packs, ATT&CK usage, the credential vault and
// schedules as host tools so a chat/task agent can act on a natural-language
// request instead of only the UI doing it.

func (s *Server) agentBridgeTools() []actool.CoreTool {
	return []actool.CoreTool{
		s.toolOsintEngagements(),
		s.toolOsintRun(),
		s.toolPacksList(),
		s.toolPackSetEnabled(),
		s.toolAttackUsage(),
		s.toolCredentialGet(),
		s.toolScheduleCreate(),
		s.toolScheduleList(),
	}
}

func (s *Server) toolOsintEngagements() actool.CoreTool {
	return roTool("osint_engagements",
		"교전 범위(RoE)와 인물 조사 허용 여부를 나열한다. OSINT 실행에는 허용된 engagement_id가 필요하다.",
		objSchema(map[string]any{}),
		func(ctx context.Context, _ json.RawMessage) (actool.Result, error) {
			rows, err := s.m.pg.ListEngagementScopes(ctx)
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			out := make([]map[string]any, 0, len(rows))
			for _, r := range rows {
				out = append(out, map[string]any{
					"id": r.ID, "name": r.Name, "person_osint_allowed": r.PersonOSINT,
					"targets": r.Targets, "starts_at": r.StartsAt, "ends_at": r.EndsAt,
				})
			}
			raw, _ := json.Marshal(out)
			return actool.Text(string(raw)), nil
		})
}

func (s *Server) toolOsintRun() actool.CoreTool {
	return wrTool("osint_run",
		"OSINT 수집기를 실행한다. kind=handle|email|contact|profile, value=대상 값. "+
			"교전의 person_osint_allowed 가 true 여야 하며 engagement_id 필수. 결과는 후보로 저장된다.",
		objSchema(map[string]any{
			"engagement_id": map[string]any{"type": "integer", "description": "교전 범위 id (osint_engagements 로 확인)"},
			"kind":          strParam("handle | email | contact | profile"),
			"value":         strParam("수집 대상 값"),
			"limit":         map[string]any{"type": "integer", "description": "최대 결과 수(기본 10)"},
		}, "engagement_id", "kind", "value"),
		func(ctx context.Context, raw json.RawMessage) (actool.Result, error) {
			var in struct {
				EngagementID int64  `json:"engagement_id"`
				Kind         string `json:"kind"`
				Value        string `json:"value"`
				Limit        int    `json:"limit"`
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				return actool.Errorf("bad args: " + err.Error()), nil
			}
			allowed, err := s.scopeAllowsPerson(ctx, in.EngagementID)
			if err != nil || !allowed {
				return actool.Errorf("이 교전은 인물 조사가 허용되지 않았습니다"), nil
			}
			if in.Limit <= 0 || in.Limit > 40 {
				in.Limit = 10
			}
			collector := struct {
				EngagementID int64  `json:"engagement_id"`
				Kind         string `json:"kind"`
				Value        string `json:"value"`
				Limit        int    `json:"limit"`
			}{in.EngagementID, in.Kind, strings.TrimSpace(in.Value), in.Limit}
			var cands []*db.OsintCandidate
			switch in.Kind {
			case "handle":
				cands, err = s.collectHandle(ctx, collector)
			case "email":
				cands, err = s.collectEmail(ctx, collector)
			case "contact":
				cands, err = s.collectContact(ctx, collector)
			case "profile":
				cands, err = s.collectProfile(ctx, collector)
			default:
				return actool.Errorf("unknown kind"), nil
			}
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			raw2, _ := json.Marshal(cands)
			return actool.Text(fmt.Sprintf("candidates=%d\n%s", len(cands), string(raw2))), nil
		})
}

func (s *Server) toolPacksList() actool.CoreTool {
	return roTool("packs_list",
		"설치된 소스 팩과 상태(설치/활성/고정/크기/저장경로)를 조회한다.",
		objSchema(map[string]any{}),
		func(_ context.Context, _ json.RawMessage) (actool.Result, error) {
			reg := s.loadPackRegistry()
			out := make([]map[string]any, 0, len(defaultPackManifest))
			for _, m := range defaultPackManifest {
				v := map[string]any{"id": m.ID, "version": m.Version, "size": m.Size, "license": m.License,
					"installed": false, "enabled": false, "pinned": false}
				if ps, ok := reg.Packs[m.ID]; ok {
					v["installed"] = true
					v["enabled"] = ps.Enabled
					v["pinned"] = ps.Pinned
					v["bytes"] = ps.Bytes
				}
				out = append(out, v)
			}
			raw, _ := json.Marshal(map[string]any{"packs": out, "dir": reg.Settings.Dir})
			return actool.Text(string(raw)), nil
		})
}

func (s *Server) toolPackSetEnabled() actool.CoreTool {
	return wrTool("packs_enable",
		"설치된 팩을 켜거나 끈다. 설치·삭제는 사람이 UI/API로만 한다. 동의 필요 팩은 consent 없이 켤 수 없다.",
		objSchema(map[string]any{
			"id":      strParam("팩 id"),
			"enabled": map[string]any{"type": "boolean", "description": "true=활성, false=비활성"},
		}, "id", "enabled"),
		func(_ context.Context, raw json.RawMessage) (actool.Result, error) {
			var in struct {
				ID      string `json:"id"`
				Enabled bool   `json:"enabled"`
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				return actool.Errorf(err.Error()), nil
			}
			reg := s.loadPackRegistry()
			ps, ok := reg.Packs[in.ID]
			if !ok {
				return actool.Errorf("pack is not installed"), nil
			}
			if in.Enabled {
				for _, m := range defaultPackManifest {
					if m.ID == in.ID && m.Consent && !ps.Consent {
						return actool.Errorf("consent required: UI/API에서 동의 후 활성화해야 합니다"), nil
					}
				}
			}
			ps.Enabled = in.Enabled
			ps.LastUsedAt = time.Now().UTC()
			reg.Packs[in.ID] = ps
			if !in.Enabled && reg.Settings.AutoDeleteOnDisable {
				_ = os.RemoveAll(filepath.Join(reg.Settings.Dir, in.ID))
				delete(reg.Packs, in.ID)
			}
			s.evictIfNeeded(&reg)
			if err := s.savePackRegistry(reg); err != nil {
				return actool.Errorf(err.Error()), nil
			}
			return actool.Text(fmt.Sprintf("pack %s enabled=%v", in.ID, in.Enabled)), nil
		})
}

func (s *Server) toolAttackUsage() actool.CoreTool {
	return roTool("attack_usage",
		"이번 교전(또는 지정 task)의 발견을 ATT&CK 기법으로 집계해 반환한다.",
		objSchema(map[string]any{"task_id": strParam("선택: 태스크 id")}),
		func(ctx context.Context, raw json.RawMessage) (actool.Result, error) {
			var in struct {
				TaskID string `json:"task_id"`
			}
			_ = json.Unmarshal(raw, &in)
			var taskID int64
			if strings.TrimSpace(in.TaskID) != "" {
				taskID, _ = parseInt64(strings.TrimSpace(in.TaskID))
			}
			data, err := s.attackUsageData(ctx, taskID)
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			out, _ := json.Marshal(data)
			return actool.Text(string(out)), nil
		})
}

func (s *Server) toolCredentialGet() actool.CoreTool {
	return wrTool("credential_get",
		"자격증명 보관소에서 비밀값을 복호화해 반환한다(감사 로그 기록). id 또는 name 으로 조회한다.",
		objSchema(map[string]any{
			"id":   strParam("자격증명 id"),
			"name": strParam("자격증명 이름(정확히 일치)"),
		}),
		func(ctx context.Context, raw json.RawMessage) (actool.Result, error) {
			var in struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				return actool.Errorf(err.Error()), nil
			}
			var enc, name, username, hint string
			var id int64
			var err error
			if strings.TrimSpace(in.ID) != "" {
				id, _ = parseInt64(strings.TrimSpace(in.ID))
				err = s.m.pg.QueryRowContext(ctx, `SELECT id,name,username,secret_enc,hint FROM credential_entry WHERE id=$1`, id).
					Scan(&id, &name, &username, &enc, &hint)
			} else {
				err = s.m.pg.QueryRowContext(ctx, `SELECT id,name,username,secret_enc,hint FROM credential_entry WHERE name=$1 ORDER BY id DESC LIMIT 1`, in.Name).
					Scan(&id, &name, &username, &enc, &hint)
			}
			if err != nil {
				return actool.Errorf("credential not found"), nil
			}
			plain, derr := s.decryptSecret(enc)
			if derr != nil {
				return actool.Errorf("decrypt failed"), nil
			}
			log.Printf("[audit] credential_get id=%d name=%q", id, name)
			out, _ := json.Marshal(map[string]any{"id": id, "name": name, "username": username, "secret": plain, "hint": hint})
			return actool.Text(string(out)), nil
		})
}

func (s *Server) toolScheduleCreate() actool.CoreTool {
	return wrTool("schedule_create",
		"태스크에 반복 스케줄을 등록한다. 서버 ticker가 간격마다 목표를 추가해 태스크를 재실행한다.",
		objSchema(map[string]any{
			"task_id":          strParam("태스크 id"),
			"goal":             strParam("매 실행 시 추가할 목표 텍스트"),
			"interval_minutes": map[string]any{"type": "integer", "description": "실행 간격(분, 기본 60)"},
			"name":             strParam("선택: 스케줄 이름"),
		}, "task_id", "goal"),
		func(ctx context.Context, raw json.RawMessage) (actool.Result, error) {
			var in struct {
				TaskID   string `json:"task_id"`
				Goal     string `json:"goal"`
				Interval int    `json:"interval_minutes"`
				Name     string `json:"name"`
			}
			if err := json.Unmarshal(raw, &in); err != nil {
				return actool.Errorf(err.Error()), nil
			}
			in.TaskID = strings.TrimSpace(in.TaskID)
			in.Goal = strings.TrimSpace(in.Goal)
			if in.TaskID == "" || in.Goal == "" {
				return actool.Errorf("task_id and goal are required"), nil
			}
			if in.Interval < 1 {
				in.Interval = 60
			}
			next := time.Now().UTC().Add(time.Duration(in.Interval) * time.Minute)
			var id int64
			if err := s.m.pg.QueryRowContext(ctx, `
INSERT INTO task_schedule(task_id,name,goal,interval_min,enabled,next_run_at)
VALUES($1,$2,$3,$4,true,$5) RETURNING id`, in.TaskID, in.Name, in.Goal, in.Interval, next).Scan(&id); err != nil {
				return actool.Errorf(err.Error()), nil
			}
			return actool.Text(fmt.Sprintf("schedule created id=%d every %dm", id, in.Interval)), nil
		})
}

func (s *Server) toolScheduleList() actool.CoreTool {
	return roTool("schedule_list", "등록된 스케줄 목록을 조회한다.", objSchema(map[string]any{}),
		func(ctx context.Context, _ json.RawMessage) (actool.Result, error) {
			rows, err := s.m.pg.QueryContext(ctx, `
SELECT id, task_id, name, goal, interval_min, enabled, next_run_at FROM task_schedule ORDER BY created_at DESC`)
			if err != nil {
				return actool.Errorf(err.Error()), nil
			}
			defer rows.Close()
			out := make([]map[string]any, 0, 16)
			for rows.Next() {
				var id, intervalMin int64
				var taskID, name, goal string
				var enabled bool
				var next time.Time
				if rows.Scan(&id, &taskID, &name, &goal, &intervalMin, &enabled, &next) != nil {
					continue
				}
				out = append(out, map[string]any{"id": id, "task_id": taskID, "name": name, "goal": goal,
					"interval_minutes": intervalMin, "enabled": enabled, "next_run_at": next})
			}
			raw, _ := json.Marshal(out)
			return actool.Text(string(raw)), nil
		})
}
