package api

import (
	"database/sql"

	"github.com/boomerang-backup/boomerang/internal/store"
)

type jobJSON struct {
	ID         string  `json:"id"`
	TargetType string  `json:"targetType"`
	TargetID   string  `json:"targetId"`
	Kind       string  `json:"kind"`
	Status     string  `json:"status"`
	Error      string  `json:"error"`
	StartedAt  *string `json:"startedAt,omitempty"`
	FinishedAt *string `json:"finishedAt,omitempty"`
	CreatedAt  string  `json:"createdAt"`
}

func nullStringPtr(ns sql.NullString) *string {
	if !ns.Valid || ns.String == "" {
		return nil
	}
	s := ns.String
	return &s
}

func jobToJSON(j store.Job) jobJSON {
	return jobJSON{
		ID:         j.ID,
		TargetType: j.TargetType,
		TargetID:   j.TargetID,
		Kind:       j.Kind,
		Status:     j.Status,
		Error:      j.Error,
		StartedAt:  nullStringPtr(j.StartedAt),
		FinishedAt: nullStringPtr(j.FinishedAt),
		CreatedAt:  j.CreatedAt,
	}
}

func jobsToJSON(list []store.Job) []jobJSON {
	if list == nil {
		list = []store.Job{}
	}
	out := make([]jobJSON, 0, len(list))
	for _, j := range list {
		out = append(out, jobToJSON(j))
	}
	return out
}
