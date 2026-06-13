package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/worryyy/devops-platform/platform/server/internal/release"
)

func (s *Store) AddEvent(ctx context.Context, releaseID string, status release.Status, message string, detail map[string]interface{}) error {
	var payload jsonBytes
	if detail != nil {
		encoded, err := json.Marshal(detail)
		if err != nil {
			return fmt.Errorf("marshal release event detail: %w", err)
		}
		payload = jsonBytes(encoded)
	}

	record := releaseEventRecord{
		ReleaseID: releaseID,
		Status:    string(status),
		Message:   message,
		Detail:    payload,
	}
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("insert release event: %w", err)
	}
	return nil
}

func (s *Store) ListEvents(ctx context.Context, releaseID string) ([]release.Event, error) {
	var records []releaseEventRecord
	err := s.db.WithContext(ctx).
		Where("release_id = ?", releaseID).
		Order("id asc").
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("list release events: %w", err)
	}

	var events []release.Event
	for _, record := range records {
		event, err := record.toEvent()
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if events == nil {
		events = []release.Event{}
	}
	return events, nil
}
