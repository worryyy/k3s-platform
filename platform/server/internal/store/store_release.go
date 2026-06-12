package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/worryyy/k3s-platform/platform/server/internal/pkg/platformerr"
	"github.com/worryyy/k3s-platform/platform/server/internal/release"
	"gorm.io/gorm"
)

func (s *Store) CreateRelease(ctx context.Context, input release.CreateReleaseInput) error {
	record := newReleaseRecord(input)
	if err := s.db.WithContext(ctx).Create(&record).Error; err != nil {
		return fmt.Errorf("create release: %w", err)
	}
	return nil
}

func (s *Store) GetRelease(ctx context.Context, id string) (release.Release, error) {
	var record releaseRecord
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return release.Release{}, platformerr.ErrNotFound
		}
		return release.Release{}, fmt.Errorf("get release: %w", err)
	}
	return record.toRelease(), nil
}

func (s *Store) ListReleasesByService(ctx context.Context, serviceName string, limit int) ([]release.Release, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var records []releaseRecord
	err := s.db.WithContext(ctx).
		Where("service_name = ?", serviceName).
		Order("created_at desc").
		Limit(limit).
		Find(&records).Error
	if err != nil {
		return nil, fmt.Errorf("list releases by service: %w", err)
	}

	var releases []release.Release
	for _, record := range records {
		releases = append(releases, record.toRelease())
	}
	if releases == nil {
		releases = []release.Release{}
	}
	return releases, nil
}

func (s *Store) UpdateReleaseStatus(ctx context.Context, id string, status release.Status, errorMessage *string) error {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":     string(status),
		"updated_at": now,
	}
	if errorMessage != nil {
		updates["error_message"] = *errorMessage
	}
	if release.IsTerminal(status) {
		updates["finished_at"] = gorm.Expr("coalesce(finished_at, ?)", now)
	}
	result := s.db.WithContext(ctx).Model(&releaseRecord{}).Where("id = ?", id).Updates(updates)
	return requireRowsAffected(result, "release", "update release status")
}

func (s *Store) UpdateReleaseStatusIfCurrent(ctx context.Context, id string, from release.Status, to release.Status) (bool, error) {
	now := time.Now().UTC()
	updates := map[string]interface{}{
		"status":     string(to),
		"updated_at": now,
	}
	if release.IsTerminal(to) {
		updates["finished_at"] = gorm.Expr("coalesce(finished_at, ?)", now)
	}
	result := s.db.WithContext(ctx).
		Model(&releaseRecord{}).
		Where("id = ? and status = ?", id, string(from)).
		Updates(updates)
	if result.Error != nil {
		return false, fmt.Errorf("update release status if current: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

func (s *Store) SetJenkinsBuild(ctx context.Context, id, jobName string, buildNumber int) error {
	result := s.db.WithContext(ctx).Model(&releaseRecord{}).Where("id = ?", id).Updates(map[string]interface{}{
		"jenkins_job":          jobName,
		"jenkins_build_number": buildNumber,
		"updated_at":           time.Now().UTC(),
	})
	return requireRowsAffected(result, "release", "set jenkins build")
}

func (s *Store) SetGitOpsRevision(ctx context.Context, id, commitSHA string) error {
	updates := map[string]interface{}{
		"commit_sha": nil,
		"updated_at": time.Now().UTC(),
	}
	if commitSHA != "" {
		updates["commit_sha"] = commitSHA
	}
	result := s.db.WithContext(ctx).Model(&releaseRecord{}).Where("id = ?", id).Updates(updates)
	return requireRowsAffected(result, "release", "set gitops revision")
}

func (s *Store) SetImage(ctx context.Context, id, repo, tag, digest string) error {
	updates := map[string]interface{}{
		"updated_at": time.Now().UTC(),
	}
	if repo != "" {
		updates["image_repo"] = repo
	}
	if tag != "" {
		updates["image_tag"] = tag
	}
	if digest != "" {
		updates["image_digest"] = digest
	}
	result := s.db.WithContext(ctx).Model(&releaseRecord{}).Where("id = ?", id).Updates(updates)
	return requireRowsAffected(result, "release", "set image")
}

func requireRowsAffected(result *gorm.DB, entity, operation string) error {
	if result.Error != nil {
		return fmt.Errorf("%s: %w", operation, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("%s: %w", entity, platformerr.ErrNotFound)
	}
	return nil
}
