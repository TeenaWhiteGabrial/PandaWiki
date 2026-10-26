package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/chaitin/panda-wiki/domain"
	"github.com/labstack/echo/v4"
)

// Scoped managers are checked for every target, including destructive descendants
// and move destinations. Unknown write routes are denied rather than granted KB-wide.
func (m *JWTMiddleware) validateScopedDocumentOperation(c echo.Context, kbID, userID string) error {
	access := domain.DocumentManagementAccessFromContext(c.Request().Context(), kbID, userID)
	if access == nil {
		var err error
		access, err = m.userAccessRepo.DocumentManagementAccess(c.Request().Context(), kbID, userID)
		if err != nil {
			return err
		}
		c.SetRequest(c.Request().WithContext(domain.WithDocumentManagementAccess(c.Request().Context(), kbID, userID, access)))
	}
	if access.Global {
		return nil
	}
	path := c.Path()
	denied := fmt.Errorf("没有此文档或目录范围的管理权限")
	if strings.HasPrefix(path, "/api/v1/file/upload/zip/tasks/:id") {
		task, err := m.userAccessRepo.ManagementZipTarget(c.Request().Context(), c.Param("id"))
		if err != nil {
			return err
		}
		if task.KBID != kbID || task.CreatedBy != userID || !access.CanCreateInNav(task.ParentID, task.NavID, domain.NodeTypeFolder) {
			return denied
		}
		return nil
	}
	if strings.HasPrefix(path, "/api/v1/file/folder-download/:id") {
		task, err := m.userAccessRepo.ManagementDownloadTarget(c.Request().Context(), c.Param("id"))
		if err != nil {
			return err
		}
		canManage := false
		if strings.HasPrefix(task.NodeID, "nav:") {
			canManage = access.CanManageNavSubtree(strings.TrimPrefix(task.NodeID, "nav:"))
		} else {
			canManage = access.CanManageSubtree(task.NodeID)
		}
		if task.KBID != kbID || task.CreatedBy != userID || !canManage {
			return denied
		}
		return nil
	}
	if c.Request().Method == http.MethodGet {
		return nil
	}
	var request struct {
		ID         string          `json:"id"`
		IDs        []string        `json:"ids"`
		NodeIDs    []string        `json:"node_ids"`
		NodeID     string          `json:"node_id"`
		ReleaseID  string          `json:"release_id"`
		ParentID   string          `json:"parent_id"`
		NavID      string          `json:"nav_id"`
		Type       domain.NodeType `json:"type"`
		TargetType string          `json:"target_type"`
	}
	if strings.HasPrefix(c.Request().Header.Get("Content-Type"), "multipart/form-data") {
		request.ParentID = c.FormValue("parent_id")
		request.NavID = c.FormValue("nav_id")
		request.ID = c.FormValue("id")
	} else if c.Request().Method == http.MethodDelete {
		request.ID = c.QueryParam("id")
	} else {
		body, err := io.ReadAll(c.Request().Body)
		if err != nil {
			return err
		}
		c.Request().Body = io.NopCloser(bytes.NewReader(body))
		if err := json.Unmarshal(body, &request); err != nil {
			return err
		}
	}
	switch path {
	case "/api/v1/node":
		if access.CanCreateInNav(request.ParentID, request.NavID, request.Type) {
			return nil
		}
		return denied
	case "/api/v1/knowledge_base/release":
		// An empty selection means publishing the entire knowledge base.
		if len(request.NodeIDs) == 0 {
			return denied
		}
	case "/api/v1/node/permission/audience":
		if len(request.IDs) == 0 || (request.TargetType != "node" && request.TargetType != "nav") {
			return denied
		}
		for _, id := range request.IDs {
			if request.TargetType == "nav" {
				if !access.CanManageNav(id) {
					return denied
				}
			} else if !access.CanManageNode(id) {
				return denied
			}
		}
		return nil
	case "/api/v1/node/management":
		if len(request.IDs) == 0 {
			return denied
		}
		for _, id := range request.IDs {
			if !access.CanManageNode(id) {
				return denied
			}
		}
		return nil
	case "/api/v1/nav/management":
		if access.CanManageNav(request.ID) {
			return nil
		}
		return denied
	case "/api/v1/node/move/nav":
		return denied
	case "/api/v1/nav/update", "/api/v1/nav/permission", "/api/v1/nav/move":
		if access.CanManageNav(request.ID) {
			return nil
		}
		return denied
	case "/api/v1/nav/delete":
		request.ID = c.QueryParam("id")
		if access.CanManageNavSubtree(request.ID) {
			return nil
		}
		return denied
	case "/api/v1/node/release/restore":
		request.ReleaseID = request.ID
		nodeID, err := m.userAccessRepo.GetReleaseNodeForManagement(c.Request().Context(), kbID, request.ReleaseID)
		if err != nil {
			return err
		}
		request.ID = nodeID
	case "/api/v1/file/upload/zip/presign":
		return nil
	case "/api/v1/file/upload/zip/tasks", "/api/v1/file/import/zip", "/api/v1/file/import/zip/commit":
		if !access.CanCreateInNav(request.ParentID, request.NavID, domain.NodeTypeFolder) {
			return denied
		}
		return nil
	case "/api/v1/file/folder-download":
		if request.NavID != "" {
			if access.CanManageNavSubtree(request.NavID) {
				return nil
			}
			return denied
		}
		if access.CanManageSubtree(request.NodeID) {
			return nil
		}
		return denied
	case "/api/v1/node/detail", "/api/v1/node/summary", "/api/v1/node/summary/stream", "/api/v1/node/action", "/api/v1/node/move", "/api/v1/node/batch_move", "/api/v1/node/restudy", "/api/v1/node/learning/cancel", "/api/v1/node/permission/edit":
	default:
		return denied
	}

	ids := append(request.IDs, request.NodeIDs...)
	if request.ID != "" {
		ids = append(ids, request.ID)
	}
	if request.NodeID != "" {
		ids = append(ids, request.NodeID)
	}
	if len(ids) == 0 {
		return denied
	}
	destructive := path == "/api/v1/node/action" || path == "/api/v1/node/move" || path == "/api/v1/node/batch_move"
	for _, id := range ids {
		if !access.CanManageNode(id) || (destructive && !access.CanManageSubtree(id)) {
			return denied
		}
		// Publishing ZIP placeholders expands new folders beside the placeholder.
		if path == "/api/v1/knowledge_base/release" && access.Nodes[id].Meta.ArchivePending && !access.CanCreateInNav(access.Nodes[id].ParentID, access.Nodes[id].NavId, domain.NodeTypeFolder) {
			return denied
		}
		if path == "/api/v1/node/detail" && request.NavID != "" && access.Nodes[id].NavId != request.NavID {
			return denied
		}
		if path == "/api/v1/node/move" || path == "/api/v1/node/batch_move" {
			node := access.Nodes[id]
			navID := request.NavID
			if navID == "" {
				navID = node.NavId
			}
			if !access.CanCreateInNav(request.ParentID, navID, node.Type) {
				return denied
			}
		}
	}
	return nil
}
