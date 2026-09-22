package pg

import (
	"context"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/store/pg"
)

type ZipUploadRepository struct {
	db     *pg.DB
	logger *log.Logger
}

func NewZipUploadRepository(db *pg.DB, logger *log.Logger) *ZipUploadRepository {
	return &ZipUploadRepository{db: db, logger: logger.WithModule("repo.pg.zip_upload")}
}
func (r *ZipUploadRepository) Create(ctx context.Context, task *domain.ZipUploadTask) error {
	return r.db.WithContext(ctx).Create(task).Error
}
func (r *ZipUploadRepository) Get(ctx context.Context, id string) (*domain.ZipUploadTask, error) {
	var t domain.ZipUploadTask
	if err := r.db.WithContext(ctx).First(&t, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &t, nil
}
func (r *ZipUploadRepository) List(ctx context.Context, kbID, userID string) ([]*domain.ZipUploadTask, error) {
	var ts []*domain.ZipUploadTask
	err := r.db.WithContext(ctx).Where("kb_id = ? AND created_by = ?", kbID, userID).Order("created_at DESC").Limit(100).Find(&ts).Error
	return ts, err
}
func (r *ZipUploadRepository) Update(ctx context.Context, task *domain.ZipUploadTask) error {
	return r.db.WithContext(ctx).Save(task).Error
}
