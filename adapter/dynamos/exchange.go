package dynamos

import (
	"encoding/json"
	"strings"

	"github.com/xe-rx/CYPHdev/engine"
)

const jobPrefix = "/agents/jobs/"

type User struct {
	ID       string `json:"id"`
	UserName string `json:"user_name"`
}

type CompositionRequest struct {
	ArchetypeID   string   `json:"archetype_id"`
	RequestType   string   `json:"request_type"`
	Role          string   `json:"role"`
	User          User     `json:"user"`
	DataProviders []string `json:"data_providers"`
	JobName       string   `json:"job_name"`
	LocalJobName  string   `json:"local_job_name"`
}

type Exchange struct {
	Key         string
	Revision    int64
	Steward     string
	User        string
	Archetype   string
	RequestType string
	Role        string
	Request     CompositionRequest
}

// job record from startup snapshot seen as exchange with its snapshot revisions revision
func ParseExchange(e engine.Event) (Exchange, bool, error) {
	if e.Kind != engine.Put && e.Kind != engine.Snapshot {
		return Exchange{}, false, nil
	}
	steward, user, ok := jobRecordParts(e.Key)
	if !ok {
		return Exchange{}, false, nil
	}
	var cr CompositionRequest
	if err := json.Unmarshal(e.Value, &cr); err != nil {
		return Exchange{}, false, err
	}
	if cr.ArchetypeID == "" || cr.User.UserName == "" {
		return Exchange{}, false, nil
	}
	return Exchange{
		Key:         e.Key,
		Revision:    e.ModRevision,
		Steward:     steward,
		User:        user,
		Archetype:   cr.ArchetypeID,
		RequestType: cr.RequestType,
		Role:        cr.Role,
		Request:     cr,
	}, true, nil
}

func jobRecordParts(key string) (steward, user string, ok bool) {
	if !strings.HasPrefix(key, jobPrefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(key, jobPrefix)
	parts := strings.Split(rest, "/")
	if len(parts) != 3 {
		return "", "", false
	}
	if parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", false
	}
	if parts[1] == "queueInfo" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func Exchanges(dir string) ([]Exchange, error) {
	var out []Exchange
	for e, err := range engine.ReadEvents(dir) {
		if err != nil {
			return nil, err
		}
		x, ok, perr := ParseExchange(e)
		if perr != nil {
			return nil, perr
		}
		if ok {
			out = append(out, x)
		}
	}
	return out, nil
}
