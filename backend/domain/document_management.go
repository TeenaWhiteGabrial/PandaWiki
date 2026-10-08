package domain

import "slices"

const (
	ManagementSubjectLevel      = "permission_level"
	ManagementSubjectDepartment = "department"
	ManagementSubjectUser       = "user"
	ManagementScopeTree         = "current_and_descendants"
	ManagementScopeCurrent      = "current"
)

// Every grant has its own directory scope. Any matching grant authorizes access.
type DocumentManagementGrant struct {
	SubjectType    string   `json:"subject_type"`
	SubjectIDs     []string `json:"subject_ids"`
	DirectoryScope string   `json:"directory_scope"`
}

func (g DocumentManagementGrant) Matches(access *EnterpriseAccessContext) bool {
	if access == nil || access.PrincipalID == "" {
		return false
	}
	switch g.SubjectType {
	case ManagementSubjectUser:
		return slices.Contains(g.SubjectIDs, access.PrincipalID)
	case ManagementSubjectLevel:
		return access.PermissionLevelID != "" && slices.Contains(g.SubjectIDs, access.PermissionLevelID)
	case ManagementSubjectDepartment:
		for _, id := range access.DepartmentIDs {
			if slices.Contains(g.SubjectIDs, id) {
				return true
			}
		}
	}
	return false
}

type DocumentManagementAccess struct {
	Global    bool
	Principal *EnterpriseAccessContext
	Nodes     map[string]*Node
	Navs      map[string]*Nav
}

func (a *DocumentManagementAccess) CanManageNode(id string) bool {
	if a == nil {
		return false
	}
	node, exists := a.Nodes[id]
	if !exists {
		return false
	}
	if a.Global {
		return true
	}
	return a.canManageAt(node, node.ParentID, false)
}

func (a *DocumentManagementAccess) matchesGrants(grants []DocumentManagementGrant, distance int, nodeType NodeType) bool {
	for _, grant := range grants {
		if grant.Matches(a.Principal) && (distance == 0 || grant.DirectoryScope == "" || grant.DirectoryScope == ManagementScopeTree || (distance == 1 && nodeType == NodeTypeDocument)) {
			return true
		}
	}
	return false
}

func (a *DocumentManagementAccess) CanManageNav(id string) bool {
	if a == nil {
		return false
	}
	nav, ok := a.Navs[id]
	return ok && (a.Global || a.matchesGrants(nav.Permissions.ManagementGrants, 0, NodeTypeFolder))
}

func (a *DocumentManagementAccess) CanManageNavSubtree(id string) bool {
	if !a.CanManageNav(id) {
		return false
	}
	for nodeID, node := range a.Nodes {
		if node.NavId == id && !a.CanManageNode(nodeID) {
			return false
		}
	}
	return true
}

// Top-level nodes use either an empty parent or the navigation ID as parent.
func (a *DocumentManagementAccess) CanCreateInNav(parentID, navID string, nodeType NodeType) bool {
	if a == nil {
		return false
	}
	if parentID == "" || parentID == navID {
		nav, ok := a.Navs[navID]
		return ok && (a.Global || a.matchesGrants(nav.Permissions.ManagementGrants, 1, nodeType))
	}
	parent, ok := a.Nodes[parentID]
	if !ok || (navID != "" && parent.NavId != navID) {
		return false
	}
	return a.CanCreate(parentID, nodeType)
}

func (a *DocumentManagementAccess) CanCreate(parentID string, nodeType NodeType) bool {
	if a == nil {
		return false
	}
	if a.Global {
		return true
	}
	if parent, ok := a.Nodes[parentID]; !ok || parent.Type != NodeTypeFolder {
		return false
	}
	return a.canManageAt(&Node{Type: nodeType}, parentID, true)
}

func (a *DocumentManagementAccess) canManageAt(node *Node, parentID string, creating bool) bool {
	current := node
	navID := node.NavId
	distance := 0
	if creating {
		current = a.Nodes[parentID]
		navID = current.NavId
		distance = 1
	}
	visited := map[string]bool{}
	for current != nil && !visited[current.ID] {
		visited[current.ID] = true
		if current.NavId != navID {
			return false
		}
		if a.matchesGrants(current.Permissions.ManagementGrants, distance, node.Type) {
			return true
		}
		parentID = current.ParentID
		current = a.Nodes[current.ParentID]
		distance++
	}
	if current == nil && (parentID == "" || parentID == navID) {
		if nav := a.Navs[navID]; nav != nil {
			return a.matchesGrants(nav.Permissions.ManagementGrants, distance, node.Type)
		}
	}
	return false
}

// Destructive operations must be authorized for every affected descendant.
func (a *DocumentManagementAccess) CanManageSubtree(id string) bool {
	if !a.CanManageNode(id) {
		return false
	}
	for childID, node := range a.Nodes {
		seen := map[string]bool{}
		for node != nil && !seen[node.ID] {
			if node.ID == id && !a.CanManageNode(childID) {
				return false
			}
			seen[node.ID] = true
			node = a.Nodes[node.ParentID]
		}
	}
	return true
}

func (a *DocumentManagementAccess) HasAnyGrant() bool {
	if a.Global {
		return true
	}
	for id := range a.Navs {
		if a.CanManageNav(id) {
			return true
		}
	}
	for id := range a.Nodes {
		if a.CanManageNode(id) {
			return true
		}
	}
	return false
}
