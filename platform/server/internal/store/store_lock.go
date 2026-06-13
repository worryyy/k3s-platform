package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/worryyy/devops-platform/platform/server/internal/pkg/platformerr"
	"gorm.io/gorm"
)

func (s *Store) AcquireReleaseLock(ctx context.Context, serviceName, environment, releaseID string, ttl time.Duration) error {
	lockedUntil := lockExpiry(ttl)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("service_name = ? and environment = ? and locked_until <= ?", serviceName, environment, time.Now().UTC()).
			Delete(&releaseLockRecord{}).Error; err != nil {
			return fmt.Errorf("delete expired release lock: %w", err)
		}

		record := releaseLockRecord{
			ServiceName: serviceName,
			Environment: environment,
			ReleaseID:   releaseID,
			LockedUntil: lockedUntil,
		}
		if err := tx.Create(&record).Error; err != nil {
			if isUniqueViolation(err) {
				return platformerr.ErrReleaseLockHeld
			}
			return fmt.Errorf("insert release lock: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) ReleaseReleaseLock(ctx context.Context, serviceName, environment, releaseID string) error {
	err := s.db.WithContext(ctx).
		Where("service_name = ? and environment = ? and release_id = ?", serviceName, environment, releaseID).
		Delete(&releaseLockRecord{}).Error
	if err != nil {
		return fmt.Errorf("release release lock: %w", err)
	}
	return nil
}

func (s *Store) ExtendReleaseLock(ctx context.Context, releaseID string, ttl time.Duration) error {
	err := s.db.WithContext(ctx).
		Model(&releaseLockRecord{}).
		Where("release_id = ?", releaseID).
		Updates(map[string]interface{}{
			"locked_until": lockExpiry(ttl),
			"updated_at":   time.Now().UTC(),
		}).Error
	if err != nil {
		return fmt.Errorf("extend release lock: %w", err)
	}
	return nil
}

func lockExpiry(ttl time.Duration) time.Time {
	if ttl <= 0 {
		ttl = time.Hour
	}
	return time.Now().UTC().Add(ttl)
}

func isUniqueViolation(err error) bool {
	var stateErr interface {
		SQLState() string
	}
	if errors.As(err, &stateErr) && stateErr.SQLState() == "23505" {
		return true
	}
	return false
}
