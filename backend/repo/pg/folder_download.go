package pg

import (
	"context"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/store/pg"
)

type FolderDownloadRepository struct {
	db     *pg.DB
	logger *log.Logger
}

func NewFolderDownloadRepository(db *pg.DB, logger *log.Logger) *FolderDownloadRepository {
	return &FolderDownloadRepository{db: db, logger: logger.WithModule("repo.pg.folder_download")}
}

func (r *FolderDownloadRepository) Create(ctx context.Context, task *domain.FolderDownloadTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}

func (r *FolderDownloadRepository) Get(ctx context.Context, id string) (*domain.FolderDownloadTask, error) {
	var task domain.FolderDownloadTask
	if err := r.db.WithContext(ctx).First(&task, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *FolderDownloadRepository) List(ctx context.Context, kbID, userID string) ([]*domain.FolderDownloadTask, error) {
	var tasks []*domain.FolderDownloadTask
	err := r.db.WithContext(ctx).Where("kb_id = ? AND created_by = ?", kbID, userID).Order("created_at DESC").Limit(100).Find(&tasks).Error
	return tasks, err
}

func (r *FolderDownloadRepository) Update(ctx context.Context, task *domain.FolderDownloadTask) error {
	return r.db.WithContext(ctx).Save(task).Error
}
