package mq

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/chaitin/panda-wiki/config"
	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/log"
	"github.com/chaitin/panda-wiki/repo/pg"
	store "github.com/chaitin/panda-wiki/store/pg"
	"github.com/chaitin/panda-wiki/store/rag"
)

type canceledLegacyMessage struct {
	data        []byte
	sequence    uint64
	publishedAt time.Time
}

type stagedIndexRAG struct {
	rag.RAGService
	dataset, document string
	uploads, deletes  int
	t                 *testing.T
}

func (r *stagedIndexRAG) ListKnowledgeBases(context.Context) ([]rag.Dataset, error) {
	return []rag.Dataset{{ID: r.dataset}}, nil
}
func (r *stagedIndexRAG) UpsertRecords(_ context.Context, req *rag.UpsertRecordsRequest) (string, error) {
	r.uploads++
	if req.DocID != "" || req.IndexVersionID == "" {
		r.t.Error("rebuild attempted to overwrite serving RAG document")
	}
	return r.document, nil
}
func (r *stagedIndexRAG) DeleteRecords(context.Context, string, []string) error {
	r.deletes++
	return nil
}

func TestRebuildSubmissionKeepsOldIndexAndDuplicateDeliveryDoesNotReupload(t *testing.T) {
	dsn := os.Getenv("PANDA_WIKI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	defer conn.Close()
	tx := db.Begin()
	defer tx.Rollback()
	ensureSafeIndexSchema(t, tx)
	l := log.NewLogger(&config.Config{})
	storeDB := &store.DB{DB: tx}
	kb := domain.KnowledgeBase{ID: uuid.NewString(), DatasetID: uuid.NewString()}
	node := domain.Node{ID: uuid.NewString(), KBID: kb.ID, Type: domain.NodeTypeDocument, Status: domain.NodeStatusPublished}
	release := domain.NodeRelease{ID: uuid.NewString(), KBID: kb.ID, NodeID: node.ID, Type: domain.NodeTypeDocument, Content: "original text", DocID: uuid.NewString()}
	task := domain.NodeLearningTask{ID: uuid.NewString(), KBID: kb.ID, NodeID: node.ID, NodeReleaseID: release.ID, Action: consts.NodeLearningTaskActionRebuild, Status: consts.NodeLearningTaskRunning}
	node.RagInfo = domain.RagInfo{Status: consts.NodeRagStatusSucceeded, ActiveDocID: release.DocID, ActiveReleaseID: release.ID, TaskID: task.ID, TaskStatus: task.Status}
	for _, row := range []any{&kb, &node, &release, &task} {
		if err := tx.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	r := &stagedIndexRAG{dataset: kb.DatasetID, document: uuid.NewString(), t: t}
	repo := pg.NewNodeRepository(storeDB, l)
	h := &RAGMQHandler{nodeRepo: repo, kbRepo: pg.NewKnowledgeBaseRepository(storeDB, &config.Config{}, l, r), rag: r, logger: l}
	body, _ := json.Marshal(domain.NodeReleaseVectorRequest{KBID: kb.ID, NodeID: node.ID, NodeReleaseID: release.ID, TaskID: task.ID, Action: "upsert"})
	for i := 0; i < 2; i++ {
		if err := h.HandleNodeContentVectorRequest(t.Context(), canceledLegacyMessage{data: body}); err != nil {
			t.Fatal(err)
		}
	}
	if r.uploads != 1 || r.deletes != 0 {
		t.Fatal("submission replaced old index or duplicated upload", r.uploads, r.deletes)
	}
	stored, _ := repo.GetNodeReleaseByID(t.Context(), release.ID)
	if stored.DocID != release.DocID {
		t.Fatal("RAG acceptance prematurely replaced serving doc ID")
	}
	update := &RagDocUpdateHandler{nodeRepo: repo, logger: l}
	event, _ := json.Marshal(domain.RagDocInfoUpdateEvent{ID: r.document, Status: "SUCCEEDED"})
	if err := update.HandleRagDocUpdate(t.Context(), canceledLegacyMessage{data: event}); err != nil {
		t.Fatal(err)
	}
	stored, _ = repo.GetNodeReleaseByID(t.Context(), release.ID)
	if stored.DocID != r.document || r.deletes != 0 {
		t.Fatal("activation did not preserve old vectors for grace-period cleanup")
	}
}

func (m canceledLegacyMessage) GetData() []byte  { return m.data }
func (m canceledLegacyMessage) GetTopic() string { return domain.VectorTaskTopic }
func (m canceledLegacyMessage) VectorQueuePosition() (uint64, time.Time, error) {
	return m.sequence, m.publishedAt, nil
}

func TestCanceledLegacyMessageIsAcknowledgableWithoutCallingRAG(t *testing.T) {
	dsn := os.Getenv("PANDA_WIKI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("local PostgreSQL required")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	conn, _ := db.DB()
	defer conn.Close()
	tx := db.Begin()
	defer tx.Rollback()
	ensureSafeIndexSchema(t, tx)
	l := log.NewLogger(&config.Config{})
	repo := pg.NewNodeRepository(&store.DB{DB: tx}, l)
	kbID := uuid.NewString()
	at := time.Now().UTC().Truncate(time.Microsecond)
	if _, _, err := repo.CancelPendingLearning(t.Context(), kbID, domain.VectorQueueBoundary{Sequence: 100, PublishedAt: at}); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(domain.NodeReleaseVectorRequest{KBID: kbID, NodeReleaseID: uuid.NewString(), Action: "upsert"})
	// Intentionally provide no KB repo, parser or RAG client: none may be called.
	h := &RAGMQHandler{nodeRepo: pg.NewNodeRepository(&store.DB{DB: tx}, l), logger: l}
	for i := 0; i < 2; i++ {
		if err := h.HandleNodeContentVectorRequest(t.Context(), canceledLegacyMessage{data: body, sequence: 99, publishedAt: at}); err != nil {
			t.Fatal("canceled message was not safely skipped/acknowledgeable", err)
		}
	}
}

func TestCanceledLearningTaskStopsParserContext(t *testing.T) {
	dsn := os.Getenv("PANDA_WIKI_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set PANDA_WIKI_TEST_PG_DSN for cancellation/storage integration")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()
	tx := db.Begin()
	defer tx.Rollback()
	ensureSafeIndexSchema(t, tx)
	task := &domain.NodeLearningTask{ID: uuid.NewString(), KBID: uuid.NewString(), NodeID: uuid.NewString(), NodeReleaseID: uuid.NewString(), Action: consts.NodeLearningTaskActionRebuild, Status: consts.NodeLearningTaskCanceled}
	if err := tx.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	repo := pg.NewNodeRepository(&store.DB{DB: tx}, log.NewLogger(&config.Config{}))
	handler := &RAGMQHandler{nodeRepo: repo}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	go handler.watchNodeLearningCancellation(ctx, cancel, task.ID)
	select {
	case <-ctx.Done():
	case <-time.After(4 * time.Second):
		t.Fatal("cancelled task did not stop its parser context")
	}

	// Extraction scope survives release storage and concurrent RAG status updates.
	node := &domain.Node{ID: task.NodeID, KBID: task.KBID, Name: "synthetic.pdf", RagInfo: domain.RagInfo{TaskID: task.ID, TaskStatus: consts.NodeLearningTaskRunning}}
	if err := tx.Create(node).Error; err != nil {
		t.Fatal(err)
	}
	release := &domain.NodeRelease{ID: task.NodeReleaseID, NodeID: node.ID, KBID: task.KBID}
	if err := tx.Create(release).Error; err != nil {
		t.Fatal(err)
	}
	result := &domain.FileExtraction{Scope: "partial", Parser: "Poppler + Tesseract", PageCount: 3, ExtractedPages: 2, OCRPages: 1}
	release.Meta.Extraction = result
	if err := repo.UpdateNodeReleaseExtractedContent(t.Context(), release.ID, "synthetic text", release.Meta); err != nil {
		t.Fatal(err)
	}
	stored, err := repo.GetNodeReleaseByID(t.Context(), release.ID)
	if err != nil || stored.Meta.Extraction == nil || stored.Meta.Extraction.Scope != "partial" || stored.Content != "synthetic text" {
		t.Fatalf("extraction was not persisted: release=%+v err=%v", stored, err)
	}
	if err := repo.RecordNodeLearningExtraction(t.Context(), task.KBID, node.ID, task.ID, result); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateNodeRagInfo(t.Context(), task.KBID, node.ID, node.RagInfo); err != nil {
		t.Fatal(err)
	}
	current, err := repo.GetNodeByID(t.Context(), node.ID)
	if err != nil || current.RagInfo.LearningProgress.Extraction.Scope != "partial" {
		t.Fatalf("RAG update lost extraction scope: %v", err)
	}
}

func ensureSafeIndexSchema(t *testing.T, tx *gorm.DB) {
	t.Helper()
	if !tx.Migrator().HasTable(&domain.NodeIndexVersion{}) {
		sql, err := os.ReadFile("../../store/pg/migration/000057_safe_index_versions.up.sql")
		if err != nil {
			t.Fatal(err)
		}
		if err := tx.Exec(string(sql)).Error; err != nil {
			t.Fatal(err)
		}
	}
}
