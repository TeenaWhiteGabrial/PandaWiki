package pg

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/chaitin/panda-wiki/consts"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/google/uuid"
)

type indexFixture struct {
	repo     *NodeRepository
	kb       domain.KnowledgeBase
	node     domain.Node
	old, new domain.NodeRelease
	task     domain.NodeLearningTask
}

func testBuildConfig(t *testing.T, f indexFixture) string {
	t.Helper()
	identity := (&domain.Model{Model: "embedding-new", Provider: "CompatiblePlatform"}).EmbeddingIndexIdentity()
	key, _ := json.Marshal(identity)
	old := (&domain.Model{Model: "embedding-old", Provider: "CompatiblePlatform"}).EmbeddingIndexIdentity()
	mode := domain.ModelModeSetting{Mode: consts.ModelSettingModeAuto, EmbeddingIndexConfig: &old, AutoProvider: &domain.AutoModeProviderSetting{Provider: "CompatiblePlatform", Models: domain.AutoModeModelSelection{Embedding: "embedding-new"}}}
	value, _ := json.Marshal(mode)
	if err := f.repo.db.Exec(`INSERT INTO system_settings(key,value) VALUES(?,?::jsonb) ON CONFLICT(key) DO UPDATE SET value=EXCLUDED.value`, consts.SystemSettingModelMode, string(value)).Error; err != nil {
		t.Fatal(err)
	}
	return string(key)
}

func newIndexFixture(t *testing.T) indexFixture {
	t.Helper()
	db := securityTestDB(t)
	f := indexFixture{repo: &NodeRepository{db: db}, kb: domain.KnowledgeBase{ID: uuid.NewString(), Name: "index fixture", DatasetID: uuid.NewString()}}
	f.node = domain.Node{ID: uuid.NewString(), KBID: f.kb.ID, Type: domain.NodeTypeDocument, Name: "fixture", Status: domain.NodeStatusPublished}
	f.old = domain.NodeRelease{ID: uuid.NewString(), KBID: f.kb.ID, NodeID: f.node.ID, Type: domain.NodeTypeDocument, Name: "old", Content: "original searchable text", DocID: uuid.NewString(), UpdatedAt: time.Now().Add(-time.Hour)}
	f.new = domain.NodeRelease{ID: uuid.NewString(), KBID: f.kb.ID, NodeID: f.node.ID, Type: domain.NodeTypeDocument, Name: "new", Content: "new candidate text", UpdatedAt: time.Now()}
	f.task = domain.NodeLearningTask{ID: uuid.NewString(), KBID: f.kb.ID, NodeID: f.node.ID, NodeReleaseID: f.new.ID, Action: consts.NodeLearningTaskActionRebuild, Status: consts.NodeLearningTaskRunning}
	f.node.RagInfo = domain.RagInfo{Status: consts.NodeRagStatusSucceeded, ActiveDocID: f.old.DocID, ActiveReleaseID: f.old.ID, TaskID: f.task.ID, TaskStatus: consts.NodeLearningTaskRunning}
	for _, item := range []any{&f.kb, &f.node, &f.old, &f.new, &f.task} {
		if err := db.Create(item).Error; err != nil {
			t.Fatal(err)
		}
	}
	return f
}

func (f indexFixture) stage(t *testing.T, id string) *domain.NodeIndexVersion {
	t.Helper()
	v, err := f.repo.PrepareNodeIndex(t.Context(), &domain.NodeIndexVersion{ID: id, KBID: f.kb.ID, NodeID: f.node.ID, NodeReleaseID: f.new.ID, DatasetID: f.kb.DatasetID, TaskID: f.task.ID})
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestIndexCandidateRetainsOldUntilSuccessfulActivation(t *testing.T) {
	f := newIndexFixture(t)
	v := f.stage(t, f.task.ID)
	candidate := uuid.NewString()
	if err := f.repo.SaveNodeIndexExtraction(t.Context(), v.ID, "new parser result", domain.NodeMeta{Extraction: &domain.FileExtraction{Scope: "full"}}); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.BindNodeIndexDocument(t.Context(), v.ID, candidate); err != nil {
		t.Fatal(err)
	}
	before, _ := f.repo.GetNodeReleaseByID(t.Context(), f.new.ID)
	if before.Content != f.new.Content {
		t.Fatal("parsed candidate content overwrote serving release before success")
	}
	for _, status := range []string{"PENDING", "RUNNING"} {
		if handled, err := f.repo.ObserveNodeIndex(t.Context(), candidate, status, "working"); err != nil || !handled {
			t.Fatal(handled, err)
		}
	}
	node, err := f.repo.GetNodeByID(t.Context(), f.node.ID)
	if err != nil || node.RagInfo.ActiveDocID != f.old.DocID {
		t.Fatal("serving pointer changed during indexing", err)
	}
	release, _ := f.repo.GetNodeReleaseByID(t.Context(), f.new.ID)
	if release.DocID != "" {
		t.Fatal("candidate leaked into serving release before success")
	}
	ids, err := f.repo.FilterServingIndexDocuments(t.Context(), []string{candidate, f.old.DocID})
	if err != nil || len(ids) != 1 || ids[0] != f.old.DocID {
		t.Fatal("candidate reached retrieval", ids, err)
	}
	fallback, err := f.repo.GetLatestDocumentReleasesWithPathsByKBID(t.Context(), f.kb.ID, 10)
	if err != nil || len(fallback) != 1 || fallback[0].ID != f.old.ID {
		t.Fatal("fallback selected new unready content", err)
	}
	// A new repository represents process restart; stage identity and binding survive.
	restarted := &NodeRepository{db: f.repo.db}
	again, err := restarted.PrepareNodeIndex(t.Context(), &domain.NodeIndexVersion{ID: v.ID, KBID: f.kb.ID, TaskID: f.task.ID})
	if err != nil || again.DocID != candidate {
		t.Fatal("retry lost candidate binding", err)
	}
	if _, err := restarted.ObserveNodeIndex(t.Context(), candidate, "SUCCEEDED", "done"); err != nil {
		t.Fatal(err)
	}
	node, _ = f.repo.GetNodeByID(t.Context(), f.node.ID)
	if node.RagInfo.ActiveDocID != candidate || node.RagInfo.ActiveReleaseID != f.new.ID || node.RagInfo.PendingIndexID != "" {
		t.Fatal("successful candidate did not atomically activate", node.RagInfo)
	}
	after, _ := f.repo.GetNodeReleaseByID(t.Context(), f.new.ID)
	if after.Content != "new parser result" || after.Meta.Extraction == nil {
		t.Fatal("successful parsing result did not commit with index")
	}
	ids, err = f.repo.FilterServingIndexDocuments(t.Context(), []string{candidate, f.old.DocID})
	if err != nil || len(ids) != 1 || ids[0] != candidate {
		t.Fatal("retired version still retrieved", ids, err)
	}
	if _, err := f.repo.ObserveNodeIndex(t.Context(), candidate, "FAILED", "late stale failure"); err != nil {
		t.Fatal(err)
	}
	node, _ = f.repo.GetNodeByID(t.Context(), f.node.ID)
	if node.RagInfo.ActiveDocID != candidate || node.RagInfo.Status != consts.NodeRagStatusSucceeded {
		t.Fatal("late event regressed active version")
	}
	retired, err := f.repo.GetNodeIndexByDocument(t.Context(), f.old.DocID)
	if err != nil || retired.Status != domain.IndexRetired {
		t.Fatal("old index not retained for delayed cleanup", err)
	}
}

func TestFailedCanceledAndTimedOutCandidatesNeverReplaceServingIndex(t *testing.T) {
	for _, terminal := range []consts.NodeLearningTaskStatus{consts.NodeLearningTaskFailed, consts.NodeLearningTaskCanceled, consts.NodeLearningTaskCancelRequested} {
		t.Run(string(terminal), func(t *testing.T) {
			f := newIndexFixture(t)
			v := f.stage(t, f.task.ID)
			doc := uuid.NewString()
			if err := f.repo.BindNodeIndexDocument(t.Context(), v.ID, doc); err != nil {
				t.Fatal(err)
			}
			if err := f.repo.db.Model(&domain.NodeLearningTask{}).Where("id = ?", f.task.ID).UpdateColumn("status", terminal).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := f.repo.ObserveNodeIndex(t.Context(), doc, "SUCCEEDED", "late completion"); err != nil {
				t.Fatal(err)
			}
			node, _ := f.repo.GetNodeByID(t.Context(), f.node.ID)
			if node.RagInfo.ActiveDocID != f.old.DocID {
				t.Fatal("terminal attempt replaced old index")
			}
		})
	}
	f := newIndexFixture(t)
	v := f.stage(t, f.task.ID)
	doc := uuid.NewString()
	_ = f.repo.BindNodeIndexDocument(t.Context(), v.ID, doc)
	if _, err := f.repo.ObserveNodeIndex(t.Context(), doc, "FAILED", "model error"); err != nil {
		t.Fatal(err)
	}
	node, _ := f.repo.GetNodeByID(t.Context(), f.node.ID)
	if node.RagInfo.ActiveDocID != f.old.DocID || node.RagInfo.Status != consts.NodeRagStatusSucceeded || node.RagInfo.TaskStatus != consts.NodeLearningTaskFailed {
		t.Fatal("failed rebuild damaged live state", node.RagInfo)
	}
}

func TestModelIndexBuildSwitchesOnlyWhenWholeSnapshotSucceeds(t *testing.T) {
	f := newIndexFixture(t)
	// No unrelated active tasks are allowed at model-build start.
	if err := f.repo.FinishNodeLearningTask(t.Context(), f.task.ID, consts.NodeLearningTaskCanceled, "", "test setup"); err != nil {
		t.Fatal(err)
	}
	other := domain.Node{ID: uuid.NewString(), KBID: f.kb.ID, Type: domain.NodeTypeDocument, RagInfo: domain.RagInfo{Status: consts.NodeRagStatusSucceeded, ActiveDocID: uuid.NewString()}}
	release := domain.NodeRelease{ID: uuid.NewString(), KBID: f.kb.ID, NodeID: other.ID, Type: domain.NodeTypeDocument, DocID: other.RagInfo.ActiveDocID, UpdatedAt: time.Now()}
	other.RagInfo.ActiveReleaseID = release.ID
	if err := f.repo.db.Create(&other).Error; err != nil {
		t.Fatal(err)
	}
	if err := f.repo.db.Create(&release).Error; err != nil {
		t.Fatal(err)
	}
	build := domain.KBIndexBuild{ID: uuid.NewString(), KBID: f.kb.ID, SourceDatasetID: f.kb.DatasetID, DatasetID: uuid.NewString(), ConfigKey: testBuildConfig(t, f)}
	if err := f.repo.StartIndexBuild(t.Context(), &build); err != nil {
		t.Fatal(err)
	}
	var versions []domain.NodeIndexVersion
	if err := f.repo.db.Where("build_id = ?", build.ID).Order("node_id").Find(&versions).Error; err != nil || len(versions) != 2 {
		t.Fatal("visible build tasks missing", err)
	}
	for i, v := range versions {
		doc := uuid.NewString()
		if err := f.repo.BindNodeIndexDocument(t.Context(), v.ID, doc); err != nil {
			t.Fatal(err)
		}
		if _, err := f.repo.ObserveNodeIndex(t.Context(), doc, "SUCCEEDED", "ready"); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			if err := f.repo.CompleteIndexBuilds(t.Context()); err != nil {
				t.Fatal(err)
			}
			var kb domain.KnowledgeBase
			_ = f.repo.db.First(&kb, "id = ?", f.kb.ID).Error
			if kb.DatasetID != f.kb.DatasetID {
				t.Fatal("partial build switched dataset")
			}
		}
	}
	// A restarted coordinator can commit all document pointers and dataset together.
	if err := (&NodeRepository{db: f.repo.db}).CompleteIndexBuilds(t.Context()); err != nil {
		t.Fatal(err)
	}
	var kb domain.KnowledgeBase
	_ = f.repo.db.First(&kb, "id = ?", f.kb.ID).Error
	if kb.DatasetID != build.DatasetID {
		t.Fatal("completed model build did not switch")
	}
	for _, v := range versions {
		stored, _ := f.repo.GetNodeIndexByDocument(t.Context(), func() string {
			var x domain.NodeIndexVersion
			_ = f.repo.db.First(&x, "id = ?", v.ID).Error
			return x.DocID
		}())
		node, _ := f.repo.GetNodeByID(t.Context(), v.NodeID)
		if stored.Status != domain.IndexActive || node.RagInfo.ActiveDocID != stored.DocID {
			t.Fatal("model cutover only partially activated document pointers")
		}
	}
}

func TestCanceledOrChangedModelBuildKeepsOriginalDataset(t *testing.T) {
	for _, scenario := range []string{"canceled", "published during build", "model changed during build"} {
		t.Run(scenario, func(t *testing.T) {
			f := newIndexFixture(t)
			_ = f.repo.FinishNodeLearningTask(t.Context(), f.task.ID, consts.NodeLearningTaskCanceled, "", "setup")
			build := domain.KBIndexBuild{ID: uuid.NewString(), KBID: f.kb.ID, SourceDatasetID: f.kb.DatasetID, DatasetID: uuid.NewString(), ConfigKey: testBuildConfig(t, f)}
			if err := f.repo.StartIndexBuild(t.Context(), &build); err != nil {
				t.Fatal(err)
			}
			var v domain.NodeIndexVersion
			_ = f.repo.db.Where("build_id = ?", build.ID).First(&v).Error
			if scenario == "canceled" {
				_ = f.repo.FinishNodeLearningTask(t.Context(), v.TaskID, consts.NodeLearningTaskCanceled, "", "cancel")
			} else {
				doc := uuid.NewString()
				_ = f.repo.BindNodeIndexDocument(t.Context(), v.ID, doc)
				if _, err := f.repo.ObserveNodeIndex(t.Context(), doc, "SUCCEEDED", "ready"); err != nil {
					t.Fatal(err)
				}
				if scenario == "published during build" {
					next := domain.NodeRelease{ID: uuid.NewString(), KBID: f.kb.ID, NodeID: f.node.ID, Type: domain.NodeTypeDocument, UpdatedAt: time.Now().Add(time.Minute)}
					if err := f.repo.db.Create(&next).Error; err != nil {
						t.Fatal(err)
					}
				} else {
					if err := f.repo.db.Exec(`UPDATE system_settings SET value=jsonb_set(value,'{auto_provider,models,embedding}','"embedding-other"') WHERE key=?`, consts.SystemSettingModelMode).Error; err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := f.repo.CompleteIndexBuilds(t.Context()); err != nil {
				t.Fatal(err)
			}
			var kb domain.KnowledgeBase
			_ = f.repo.db.First(&kb, "id = ?", f.kb.ID).Error
			if kb.DatasetID != f.kb.DatasetID {
				t.Fatal("incomplete model build switched dataset")
			}
			node, _ := f.repo.GetNodeByID(t.Context(), f.node.ID)
			if node.RagInfo.ActiveDocID != f.old.DocID {
				t.Fatal("failed model build lost original document index")
			}
			latest, err := f.repo.GetLatestIndexBuild(t.Context(), f.kb.ID)
			if err != nil || latest.Status != domain.IndexFailed {
				t.Fatal("failed build did not persist reason", err)
			}
		})
	}
}

func TestIndexBaselineAdvancesAtCutoverAndSurvivesStaleSettingsSave(t *testing.T) {
	f := newIndexFixture(t)
	key := testBuildConfig(t, f)
	_ = f.repo.FinishNodeLearningTask(t.Context(), f.task.ID, consts.NodeLearningTaskCanceled, "", "setup")
	// Represent other locally existing KBs as already switched. These records
	// exist only inside this test's rollback transaction; no real index is touched.
	var others []domain.KnowledgeBase
	_ = f.repo.db.Where("id <> ?", f.kb.ID).Find(&others).Error
	for _, kb := range others {
		b := domain.KBIndexBuild{ID: uuid.NewString(), KBID: kb.ID, SourceDatasetID: kb.DatasetID, DatasetID: kb.DatasetID, ConfigKey: key, Status: domain.IndexActive}
		if err := f.repo.db.Create(&b).Error; err != nil {
			t.Fatal(err)
		}
	}
	build := domain.KBIndexBuild{ID: uuid.NewString(), KBID: f.kb.ID, SourceDatasetID: f.kb.DatasetID, DatasetID: uuid.NewString(), ConfigKey: key}
	if err := f.repo.StartIndexBuild(t.Context(), &build); err != nil {
		t.Fatal(err)
	}
	var setting domain.SystemSetting
	_ = f.repo.db.Where("key = ?", consts.SystemSettingModelMode).First(&setting).Error
	var stale domain.ModelModeSetting
	_ = json.Unmarshal(setting.Value, &stale)
	if stale.EmbeddingIndexConfig.Model != "embedding-old" {
		t.Fatal("enqueue advanced baseline")
	}
	var v domain.NodeIndexVersion
	_ = f.repo.db.Where("build_id = ?", build.ID).First(&v).Error
	doc := uuid.NewString()
	_ = f.repo.BindNodeIndexDocument(t.Context(), v.ID, doc)
	if _, err := f.repo.ObserveNodeIndex(t.Context(), doc, "SUCCEEDED", "ready"); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.CompleteIndexBuilds(t.Context()); err != nil {
		t.Fatal(err)
	}
	settings := &SystemSettingRepo{db: f.repo.db}
	if err := settings.SaveModelModeSetting(t.Context(), stale); err != nil {
		t.Fatal(err)
	}
	stored, err := settings.GetSystemSetting(t.Context(), consts.SystemSettingModelMode)
	if err != nil {
		t.Fatal(err)
	}
	var current domain.ModelModeSetting
	_ = json.Unmarshal(stored.Value, &current)
	if current.EmbeddingIndexConfig.Model != "embedding-new" {
		t.Fatal("stale model form rolled back the applied index baseline")
	}
}

func TestModelBuildUsesPublishedSnapshotInsteadOfPendingDraft(t *testing.T) {
	f := newIndexFixture(t)
	_ = f.repo.FinishNodeLearningTask(t.Context(), f.task.ID, consts.NodeLearningTaskCanceled, "", "setup")
	publication := domain.KBRelease{ID: uuid.NewString(), KBID: f.kb.ID, CreatedAt: time.Now()}
	mapping := domain.KBReleaseNodeRelease{ID: uuid.NewString(), KBID: f.kb.ID, ReleaseID: publication.ID, NodeID: f.node.ID, NodeReleaseID: f.new.ID}
	draft := domain.NodeRelease{ID: uuid.NewString(), KBID: f.kb.ID, NodeID: f.node.ID, Type: domain.NodeTypeDocument, Content: "unpublished", UpdatedAt: time.Now().Add(time.Hour)}
	for _, row := range []any{&publication, &mapping, &draft} {
		if err := f.repo.db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	releases, err := f.repo.GetPublishedIndexReleases(t.Context(), f.kb.ID, []string{f.node.ID})
	if err != nil || len(releases) != 1 || releases[0].ID != f.new.ID {
		t.Fatal("pending draft would enter model rebuild", err)
	}
	build := domain.KBIndexBuild{ID: uuid.NewString(), KBID: f.kb.ID, SourceDatasetID: f.kb.DatasetID, DatasetID: uuid.NewString(), ConfigKey: testBuildConfig(t, f)}
	if err := f.repo.StartIndexBuild(t.Context(), &build); err != nil {
		t.Fatal(err)
	}
	var v domain.NodeIndexVersion
	_ = f.repo.db.Where("build_id = ?", build.ID).First(&v).Error
	if v.NodeReleaseID != f.new.ID {
		t.Fatal("unpublished version staged")
	}
}
