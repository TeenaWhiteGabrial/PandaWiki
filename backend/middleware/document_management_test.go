package middleware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/labstack/echo/v4"
)

func TestScopedDocumentOperationsValidateEveryTarget(t *testing.T) {
	access := &domain.DocumentManagementAccess{Principal: &domain.EnterpriseAccessContext{PrincipalID: "alice"}, Nodes: map[string]*domain.Node{
		"root":    {ID: "root", NavId: "nav", Type: domain.NodeTypeFolder, Permissions: domain.NodePermissions{ManagementGrants: []domain.DocumentManagementGrant{{SubjectType: "user", SubjectIDs: []string{"alice"}, DirectoryScope: "current"}}}},
		"doc":     {ID: "doc", NavId: "nav", Type: domain.NodeTypeDocument, ParentID: "root"},
		"sub":     {ID: "sub", NavId: "nav", Type: domain.NodeTypeFolder, ParentID: "root"},
		"outside": {ID: "outside", Type: domain.NodeTypeDocument},
		"archive": {ID: "archive", NavId: "nav", Type: domain.NodeTypeDocument, ParentID: "root", Meta: domain.NodeMeta{ArchivePending: true}},
	}}
	for _, tc := range []struct {
		path, body string
		allowed    bool
	}{
		{"/api/v1/node/detail", `{"id":"doc"}`, true},
		{"/api/v1/node/detail", `{"id":"doc","nav_id":"another-nav"}`, false},
		{"/api/v1/node/action", `{"ids":["doc","outside"]}`, false},
		{"/api/v1/node/action", `{"ids":["root"]}`, false},
		{"/api/v1/node/restudy", `{"node_ids":["doc"]}`, true},
		{"/api/v1/node", `{"parent_id":"root","nav_id":"nav","type":2}`, true},
		{"/api/v1/node", `{"parent_id":"root","nav_id":"other","type":2}`, false},
		{"/api/v1/node", `{"parent_id":"root","type":1}`, false},
		{"/api/v1/node/management", `{"ids":["doc"]}`, true},
		{"/api/v1/node/permission/audience", `{"ids":["doc"],"target_type":"node"}`, true},
		{"/api/v1/node/permission/audience", `{"ids":["doc","outside"],"target_type":"node"}`, false},
		{"/api/v1/node/permission/audience", `{"ids":[],"target_type":"node"}`, false},
		{"/api/v1/node/permission/audience", `{"ids":["doc"],"target_type":"nav"}`, false},
		{"/api/v1/node/management", `{"ids":["doc","outside"]}`, false},
		{"/api/v1/knowledge_base/release", `{"node_ids":[]}`, false},
		{"/api/v1/knowledge_base/release", `{"node_ids":["doc"]}`, true},
		{"/api/v1/knowledge_base/release", `{"node_ids":["archive"]}`, false},
		{"/api/v1/node/move", `{"id":"doc","parent_id":"root"}`, true},
		{"/api/v1/node/move", `{"id":"doc","parent_id":"sub"}`, false},
		{"/api/v1/node/unknown", `{"id":"doc"}`, false},
	} {
		t.Run(tc.path+tc.body, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(domain.WithDocumentManagementAccess(context.Background(), "kb", "alice", access))
			c := echo.New().NewContext(req, httptest.NewRecorder())
			c.SetPath(tc.path)
			err := (&JWTMiddleware{}).validateScopedDocumentOperation(c, "kb", "alice")
			if (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v error=%v", tc.allowed, err)
			}
			preserved, _ := io.ReadAll(c.Request().Body)
			if string(preserved) != tc.body {
				t.Fatal("authorization consumed the handler's request body")
			}
		})
	}
}

func TestScopedNavigationOperations(t *testing.T) {
	access := &domain.DocumentManagementAccess{
		Principal: &domain.EnterpriseAccessContext{PrincipalID: "alice"},
		Navs: map[string]*domain.Nav{
			"nav":     {ID: "nav", Permissions: domain.NodePermissions{ManagementGrants: []domain.DocumentManagementGrant{{SubjectType: "user", SubjectIDs: []string{"alice"}, DirectoryScope: "current"}}}},
			"outside": {ID: "outside"},
		},
		Nodes: map[string]*domain.Node{
			"direct": {ID: "direct", NavId: "nav", Type: domain.NodeTypeDocument},
			"sub":    {ID: "sub", NavId: "nav", Type: domain.NodeTypeFolder},
		},
	}
	for _, tc := range []struct {
		method, path, body string
		allowed            bool
	}{
		{http.MethodPatch, "/api/v1/nav/update", `{"id":"nav"}`, true},
		{http.MethodPost, "/api/v1/node/permission/audience", `{"ids":["nav"],"target_type":"nav"}`, true},
		{http.MethodPost, "/api/v1/node/permission/audience", `{"ids":["nav","outside"],"target_type":"nav"}`, false},
		{http.MethodPatch, "/api/v1/nav/permission", `{"id":"nav"}`, true},
		{http.MethodPatch, "/api/v1/nav/update", `{"id":"outside"}`, false},
		{http.MethodPatch, "/api/v1/nav/management", `{"id":"nav"}`, true},
		{http.MethodPatch, "/api/v1/nav/management", `{"id":"outside"}`, false},
		{http.MethodPost, "/api/v1/node", `{"nav_id":"nav","type":2}`, true},
		{http.MethodPost, "/api/v1/node", `{"nav_id":"nav","parent_id":"nav","type":2}`, true},
		{http.MethodPost, "/api/v1/node", `{"nav_id":"nav","type":1}`, false},
		{http.MethodPost, "/api/v1/node", `{"nav_id":"outside","type":2}`, false},
		{http.MethodPost, "/api/v1/file/folder-download", `{"nav_id":"nav"}`, false},
		{http.MethodPost, "/api/v1/file/import/zip/commit", `{"nav_id":"nav"}`, false},
		{http.MethodDelete, "/api/v1/nav/delete", "", false},
	} {
		t.Run(tc.method+tc.path+tc.body, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path+"?id=nav&kb_id=kb", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req = req.WithContext(domain.WithDocumentManagementAccess(context.Background(), "kb", "alice", access))
			c := echo.New().NewContext(req, httptest.NewRecorder())
			c.SetPath(tc.path)
			if err := (&JWTMiddleware{}).validateScopedDocumentOperation(c, "kb", "alice"); (err == nil) != tc.allowed {
				t.Fatalf("allowed=%v error=%v", tc.allowed, err)
			}
		})
	}
}

func TestScopedFolderDownloadCanTargetEntireNavigation(t *testing.T) {
	access := &domain.DocumentManagementAccess{
		Principal: &domain.EnterpriseAccessContext{PrincipalID: "alice"},
		Navs: map[string]*domain.Nav{
			"nav": {ID: "nav", Permissions: domain.NodePermissions{ManagementGrants: []domain.DocumentManagementGrant{{SubjectType: "user", SubjectIDs: []string{"alice"}, DirectoryScope: domain.ManagementScopeTree}}}},
		},
		Nodes: map[string]*domain.Node{
			"direct": {ID: "direct", NavId: "nav", Type: domain.NodeTypeDocument},
			"folder": {ID: "folder", NavId: "nav", Type: domain.NodeTypeFolder, ParentID: "nav"},
			"nested": {ID: "nested", NavId: "nav", Type: domain.NodeTypeDocument, ParentID: "folder"},
		},
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/file/folder-download", strings.NewReader(`{"nav_id":"nav"}`))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(domain.WithDocumentManagementAccess(context.Background(), "kb", "alice", access))
	c := echo.New().NewContext(req, httptest.NewRecorder())
	c.SetPath("/api/v1/file/folder-download")
	if err := (&JWTMiddleware{}).validateScopedDocumentOperation(c, "kb", "alice"); err != nil {
		t.Fatalf("expected access to entire navigation to allow download, got %v", err)
	}
}
