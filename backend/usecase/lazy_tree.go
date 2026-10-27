package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	v1 "github.com/chaitin/panda-wiki/api/node/v1"
	shareV1 "github.com/chaitin/panda-wiki/api/share/v1"
	"github.com/chaitin/panda-wiki/domain"
	"github.com/chaitin/panda-wiki/repo/pg"
)

// Only the authorization graph is scanned as a whole. Names, file metadata,
// indexing progress and permission labels are hydrated for the requested level.
// This graph and all authorization decisions live for one request only.
type treeEdge struct{ id, parent, nav string }

func treeRevision(value any) string {
	raw, _ := json.Marshal(value)
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func treeParent(e treeEdge) string {
	if e.parent == e.nav {
		return ""
	}
	return e.parent
}
func selectTreeLevel(edges map[string]treeEdge, req v1.NodeListGroupNavReq, matches map[string]bool) ([]string, map[string][]string, map[string]bool) {
	parents := map[string]map[string]bool{}
	add := func(nav, parent string) {
		if parents[nav] == nil {
			parents[nav] = map[string]bool{}
		}
		parents[nav][parent] = true
	}
	if req.NavID != "" {
		add(req.NavID, req.ParentID)
	}
	// A refresh can revalidate all already-loaded levels in one request.
	for _, id := range req.Parents {
		if e, ok := edges[id]; ok {
			add(e.nav, e.id)
		} else if strings.HasPrefix(id, "nav:") {
			add(strings.TrimPrefix(id, "nav:"), "")
		}
	}
	selected := map[string]bool{}
	walk := func(id string, siblings bool) {
		seen := map[string]bool{}
		nav := edges[id].nav
		for id != "" && !seen[id] {
			seen[id] = true
			e, ok := edges[id]
			if !ok || e.nav != nav {
				break
			}
			selected[id] = true
			if siblings {
				add(e.nav, treeParent(e))
			}
			id = treeParent(e)
		}
	}
	if req.PathID != "" {
		walk(req.PathID, true)
	}
	if matches != nil {
		for id := range matches {
			walk(id, false)
		}
	}
	hasChildren := map[string]bool{}
	for id, e := range edges {
		parent := treeParent(e)
		if parent == "" {
			hasChildren["nav:"+e.nav] = true
		} else {
			hasChildren[parent] = true
		}
		if parents[e.nav][parent] {
			selected[id] = true
		}
	}
	ids := make([]string, 0, len(selected))
	for id := range selected {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	loaded := map[string][]string{}
	for nav, ps := range parents {
		for p := range ps {
			loaded[nav] = append(loaded[nav], p)
		}
		sort.Strings(loaded[nav])
	}
	return ids, loaded, hasChildren
}

func (u *NodeUsecase) GetNodeTree(ctx context.Context, req v1.NodeListGroupNavReq) ([]*v1.NodeListGroupNavResp, error) {
	done := domain.MeasureTreeLoadPhase(ctx, "access_load")
	access, err := u.enterprise.ManagementTreeAccess(ctx, req.KbId)
	done()
	if err != nil {
		return nil, err
	}
	access = access.ForEvaluation()
	var visible map[string]bool
	if !access.Global {
		visible = managementVisibleNodeIDs(access)
	}
	edges := map[string]treeEdge{}
	visibleNavs := map[string]bool{}
	for id, n := range access.Nodes {
		if access.Global || visible[id] {
			edges[id] = treeEdge{id, n.ParentID, n.NavId}
			visibleNavs[n.NavId] = true
		}
	}
	policies := map[string]any{}
	for id, n := range access.Nodes {
		policies[id] = []any{n.ParentID, n.NavId, n.Type, n.Permissions, visible[id] || access.Global}
	}
	revision := treeRevision([]any{policies, access.Navs, access.Principal, access.Global})
	var matches map[string]bool
	if req.Search != "" {
		found, err := u.nodeRepo.GetNodeListByStatus(ctx, req.KbId, req.Status, req.Search)
		if err != nil {
			return nil, err
		}
		matches = map[string]bool{}
		for _, n := range found {
			if _, ok := edges[n.ID]; ok {
				matches[n.ID] = true
			}
		}
		// Search results include authorized ancestor paths, not unrelated siblings.
		req.NavID = ""
		req.Parents = nil
		req.PathID = ""
	}
	ids, loaded, has := selectTreeLevel(edges, req, matches)
	done = domain.MeasureTreeLoadPhase(ctx, "level_metadata_query")
	nodes, err := u.nodeRepo.GetNodeListByStatus(ctx, req.KbId, "", "", ids)
	done()
	if metrics := domain.TreeLoadMetricsFromContext(ctx); metrics != nil {
		metrics.InputNodes = len(access.Nodes)
		metrics.VisibleNodes = len(nodes)
	}
	if err != nil {
		return nil, err
	}
	nodes, err = u.filterManagedNodesWithAccess(ctx, access, nodes)
	if err != nil {
		return nil, err
	}
	navs, err := u.navRepo.GetListByIds(ctx, req.KbId, req.NavIds)
	if err != nil {
		return nil, err
	}
	released, err := u.navRepo.GetReleaseList(ctx, req.KbId)
	if err != nil {
		return nil, err
	}
	releasedMap := map[string]bool{}
	for _, n := range released {
		for _, draft := range navs {
			if n.ID == draft.ID {
				releasedMap[n.ID] = n.Name == draft.Name && n.Position == draft.Position
			}
		}
	}
	result := []*v1.NodeListGroupNavResp{}
	for _, nav := range navs {
		if !access.Global && !visibleNavs[nav.ID] && !access.CanManageNav(nav.ID) {
			continue
		}
		group := &v1.NodeListGroupNavResp{TreeRevision: revision, NavID: nav.ID, NavName: nav.Name, Position: nav.Position, Permissions: nav.Permissions, CanManage: access.CanManageNav(nav.ID), CanConfigure: access.CanManageNav(nav.ID), CanCreateDocument: access.CanCreateInNav("", nav.ID, domain.NodeTypeDocument), CanCreateFolder: access.CanCreateInNav("", nav.ID, domain.NodeTypeFolder), HasChildren: has["nav:"+nav.ID], IsReleased: releasedMap[nav.ID], LoadedParents: loaded[nav.ID], List: []domain.NodeListItemResp{}}
		for _, n := range nodes {
			if n.NavId == nav.ID {
				n.HasChildren = has[n.ID]
				group.List = append(group.List, *n)
			}
		}
		var entry *domain.Node
		for id, e := range edges {
			if e.nav == nav.ID {
				group.Count++
				n := access.Nodes[id]
				if n.Type == domain.NodeTypeDocument && access.CanManageNode(id) && (entry == nil || n.Position < entry.Position || n.Position == entry.Position && n.ID < entry.ID) {
					entry = n
				}
			}
		}
		if entry != nil {
			group.EntryNodeID = entry.ID
		}
		if matches == nil || len(group.List) > 0 {
			result = append(result, group)
		}
	}
	return result, nil
}

func (u *NodeUsecase) GetShareNodeTree(ctx context.Context, kbID, userID string, req v1.NodeListGroupNavReq) ([]*shareV1.NodeListGroupNavResp, error) {
	done := domain.MeasureTreeLoadPhase(ctx, "policy_graph_query")
	graph, err := u.nodeRepo.GetNodeReleaseListByKBID(ctx, kbID, pg.TreeNodeQuery{PolicyOnly: true})
	done()
	if err != nil {
		return nil, err
	}
	navs, err := u.navRepo.GetReleaseList(ctx, kbID)
	if err != nil {
		return nil, err
	}
	principal, err := u.enterprise.BuildShareAccessContext(ctx, userID)
	if err != nil {
		return nil, err
	}
	visible, err := u.filterShareNodesWithAccess(ctx, kbID, graph, navs, principal)
	if err != nil {
		return nil, err
	}
	edges := map[string]treeEdge{}
	policies := map[string]*domain.ShareNodeListItemResp{}
	for _, n := range visible {
		edges[n.ID] = treeEdge{n.ID, n.ParentID, n.NavId}
		policies[n.ID] = n
	}
	sort.Slice(visible, func(i, j int) bool { return visible[i].ID < visible[j].ID })
	revision := treeRevision([]any{visible, navs})
	var matches map[string]bool
	if req.Search != "" {
		found, err := u.nodeRepo.GetNodeReleaseListByKBID(ctx, kbID, pg.TreeNodeQuery{PolicyOnly: true, Search: req.Search})
		if err != nil {
			return nil, err
		}
		matches = map[string]bool{}
		for _, n := range found {
			if p := policies[n.ID]; p != nil && !p.PathOnly {
				matches[n.ID] = true
			}
		}
		req.NavID = ""
		req.PathID = ""
		req.Parents = nil
	}
	if req.NavID == "" && req.ParentID != "" {
		if p, ok := edges[req.ParentID]; ok {
			req.NavID = p.nav
		}
	}
	ids, loaded, has := selectTreeLevel(edges, req, matches)
	done = domain.MeasureTreeLoadPhase(ctx, "level_metadata_query")
	nodes, err := u.nodeRepo.GetNodeReleaseListByKBID(ctx, kbID, pg.TreeNodeQuery{IDs: ids})
	done()
	if metrics := domain.TreeLoadMetricsFromContext(ctx); metrics != nil {
		metrics.InputNodes = len(graph)
		metrics.VisibleNodes = len(nodes)
	}
	if err != nil {
		return nil, err
	}
	result := []*shareV1.NodeListGroupNavResp{}
	for _, nav := range navs {
		if nav.Permissions.HasEnterpriseRule() && !u.enterprise.CanVisible(nav.Permissions, principal) {
			continue
		}
		if !has["nav:"+nav.ID] {
			continue
		}
		group := &shareV1.NodeListGroupNavResp{ViewerID: userID, TreeRevision: revision, NavID: nav.ID, NavName: nav.Name, Position: nav.Position, HasChildren: true, LoadedParents: loaded[nav.ID], List: []domain.ShareNodeListItemResp{}}
		// Stable entry navigation does not require downloading the complete tree.
		candidates := []*domain.ShareNodeListItemResp{}
		for _, n := range visible {
			if n.NavId == nav.ID {
				group.Count++
				if !n.PathOnly {
					candidates = append(candidates, n)
				}
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			if candidates[i].Type != candidates[j].Type {
				return candidates[i].Type == domain.NodeTypeDocument
			}
			if candidates[i].Position == candidates[j].Position {
				return candidates[i].ID < candidates[j].ID
			}
			return candidates[i].Position < candidates[j].Position
		})
		if len(candidates) > 0 {
			group.EntryNodeID = candidates[0].ID
		}
		for _, n := range nodes {
			if n.NavId != nav.ID {
				continue
			}
			p := policies[n.ID]
			if p == nil {
				continue
			}
			n.HasChildren = has[n.ID]
			n.PathOnly = p.PathOnly
			// ACL details and storage keys are never part of a reader tree response.
			n.Permissions = domain.NodePermissions{}
			n.Meta = domain.NodeMeta{Summary: n.Meta.Summary}
			if p.PathOnly {
				n.Emoji = ""
				n.Meta = domain.NodeMeta{}
				n.UpdatedAt = p.UpdatedAt
			}
			group.List = append(group.List, *n)
		}
		if matches == nil || len(group.List) > 0 {
			result = append(result, group)
		}
	}
	return result, nil
}

type TreeNodeStatus struct {
	ID      string            `json:"id"`
	Status  domain.NodeStatus `json:"status"`
	RagInfo domain.RagInfo    `json:"rag_info"`
}

func (u *NodeUsecase) GetTreeNodeStatuses(ctx context.Context, kbID string, ids []string) ([]TreeNodeStatus, error) {
	access, err := u.enterprise.ManagementTreeAccess(ctx, kbID)
	if err != nil {
		return nil, err
	}
	access = access.ForEvaluation()
	allowed := []string{}
	for _, id := range ids {
		if len(allowed) >= 1000 {
			break
		}
		if access.CanManageNode(id) {
			allowed = append(allowed, id)
		}
	}
	nodes, err := u.nodeRepo.GetTreeNodeStatuses(ctx, kbID, allowed)
	if err != nil {
		return nil, err
	}
	result := []TreeNodeStatus{}
	for _, n := range nodes {
		result = append(result, TreeNodeStatus{n.ID, n.Status, n.RagInfo})
	}
	return result, nil
}
