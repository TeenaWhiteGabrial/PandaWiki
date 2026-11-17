package domain

import "slices"

const (
	ManagementSubjectLevel      = "permission_level"
	ManagementSubjectDepartment = "department"
	ManagementSubjectUser       = "user"
	ManagementScopeTree         = "current_and_descendants"
	ManagementScopeCurrent      = "current"
)

// Management grants are additive: a local grant never revokes an ancestor grant.
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
	Global     bool
	Principal  *EnterpriseAccessContext
	Nodes      map[string]*Node
	Navs       map[string]*Nav
	evaluation *managementEvaluationCache
}

type managementEvaluationCache struct {
	constraints map[string][][]DocumentManagementGrant
	managed     map[string]bool
}

// ForEvaluation reuses hierarchy calculations within one read-only operation.
// The original access remains uncached so permission edits cannot reuse stale
// answers. Do not mutate Nodes, Navs or Principal while using this evaluator.
func (a *DocumentManagementAccess) ForEvaluation() *DocumentManagementAccess {
	if a == nil {
		return nil
	}
	copy := *a
	copy.evaluation = &managementEvaluationCache{
		constraints: make(map[string][][]DocumentManagementGrant),
		managed:     make(map[string]bool),
	}
	return &copy
}

func (a *DocumentManagementAccess) CanManageNode(id string) bool {
	if a == nil {
		return false
	}
	_, exists := a.Nodes[id]
	if !exists {
		return false
	}
	if a.Global {
		return true
	}
	if a.evaluation != nil {
		if allowed, found := a.evaluation.managed[id]; found {
			return allowed
		}
	}
	allowed := a.matchesAnyManagementGrant(a.ManagementGrantConstraintsForNode(id))
	if a.evaluation != nil {
		a.evaluation.managed[id] = allowed
	}
	return allowed
}

// ManagementGrantsForNode returns the management rules that apply to this node
// through its own node, ancestor folders, or navigation root. It is intended
// for permission summaries; callers must still restrict who can see the result.
func (a *DocumentManagementAccess) ManagementGrantsForNode(id string) []DocumentManagementGrant {
	var grants []DocumentManagementGrant
	for _, constraint := range a.ManagementGrantConstraintsForNode(id) {
		grants = appendApplicableManagementGrants(grants, constraint, 0, NodeTypeFolder)
	}
	return grants
}

// ManagementGrantConstraintsForNode retains the API grouping by source, but
// groups are alternatives: matching any applicable ancestor or local grant wins.
func (a *DocumentManagementAccess) ManagementGrantConstraintsForNode(id string) [][]DocumentManagementGrant {
	if a == nil {
		return nil
	}
	if a.evaluation != nil {
		if v, ok := a.evaluation.constraints[id]; ok {
			return slices.Clone(v)
		}
	}
	node := a.Nodes[id]
	if node == nil {
		return nil
	}
	result := a.managementGrantConstraints(node, node.ParentID, 0, node.Type)
	if a.evaluation != nil {
		a.evaluation.constraints[id] = slices.Clone(result)
	}
	return result
}

// Parent grants include navigation authorization, even for top-level folders.
func (a *DocumentManagementAccess) ManagementGrantConstraintsForParentNode(id string) [][]DocumentManagementGrant {
	if a == nil {
		return nil
	}
	node := a.Nodes[id]
	if node == nil {
		return nil
	}
	return a.managementGrantConstraints(a.Nodes[node.ParentID], node.ParentID, 1, node.Type, node.NavId)
}

func (a *DocumentManagementAccess) managementGrantConstraints(current *Node, parentID string, distance int, targetType NodeType, navIDOverride ...string) [][]DocumentManagementGrant {
	if a == nil {
		return nil
	}
	navID := ""
	if current != nil {
		navID = current.NavId
	}
	if len(navIDOverride) > 0 {
		navID = navIDOverride[0]
	}
	visited := map[string]bool{}
	var groups [][]DocumentManagementGrant
	for current != nil {
		if visited[current.ID] || current.NavId != navID {
			return nil
		}
		visited[current.ID] = true
		if applicable := applicableManagementGrants(current.Permissions.ManagementGrants, distance, targetType); len(applicable) > 0 {
			groups = append(groups, applicable)
		}
		parentID = current.ParentID
		if parentID == "" || parentID == navID {
			break
		}
		parent := a.Nodes[parentID]
		if parent == nil || parent.Type != NodeTypeFolder {
			return nil
		}
		current = parent
		distance++
	}
	if current == nil && parentID != "" && parentID != navID {
		return nil
	}
	// The navigation is the parent of the root node, not its alternative.
	if current != nil {
		distance++
	}
	if nav := a.Navs[navID]; nav != nil {
		if applicable := applicableManagementGrants(nav.Permissions.ManagementGrants, distance, targetType); len(applicable) > 0 {
			groups = append(groups, applicable)
		}
	}
	return groups
}

type ManagementGrantSource struct {
	ID     string                    `json:"id"`
	Name   string                    `json:"name"`
	Grants []DocumentManagementGrant `json:"grants"`
}

// ManagementGrantSourcesForNode describes grants without exposing unrelated
// content. Callers must authorize inspection of the target before returning it.
func (a *DocumentManagementAccess) ManagementGrantSourcesForNode(id string) []ManagementGrantSource {
	node := a.Nodes[id]
	if node == nil || len(a.ManagementGrantConstraintsForNode(id)) == 0 {
		return nil
	}
	var result []ManagementGrantSource
	current := node
	distance := 0
	for current != nil {
		if grants := applicableManagementGrants(current.Permissions.ManagementGrants, distance, node.Type); len(grants) > 0 {
			result = append(result, ManagementGrantSource{ID: current.ID, Name: current.Name, Grants: grants})
		}
		if current.ParentID == "" || current.ParentID == node.NavId {
			distance++
			break
		}
		current = a.Nodes[current.ParentID]
		distance++
	}
	if nav := a.Navs[node.NavId]; nav != nil {
		if grants := applicableManagementGrants(nav.Permissions.ManagementGrants, distance, node.Type); len(grants) > 0 {
			result = append(result, ManagementGrantSource{ID: "nav:" + nav.ID, Name: nav.Name, Grants: grants})
		}
	}
	return result
}

func applicableManagementGrants(grants []DocumentManagementGrant, distance int, nodeType NodeType) []DocumentManagementGrant {
	var result []DocumentManagementGrant
	for _, grant := range grants {
		if distance != 0 && grant.DirectoryScope != "" && grant.DirectoryScope != ManagementScopeTree && !(distance == 1 && nodeType == NodeTypeDocument) {
			continue
		}
		result = append(result, grant)
	}
	return result
}

func appendApplicableManagementGrants(result, grants []DocumentManagementGrant, distance int, nodeType NodeType) []DocumentManagementGrant {
	for _, grant := range applicableManagementGrants(grants, distance, nodeType) {
		duplicate := false
		for _, existing := range result {
			if existing.SubjectType == grant.SubjectType && existing.DirectoryScope == grant.DirectoryScope && slices.Equal(existing.SubjectIDs, grant.SubjectIDs) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, grant)
		}
	}
	return result
}

func (a *DocumentManagementAccess) matchesGrants(grants []DocumentManagementGrant, principal *EnterpriseAccessContext) bool {
	for _, grant := range grants {
		if grant.Matches(principal) {
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
	return ok && (a.Global || a.matchesGrants(applicableManagementGrants(nav.Permissions.ManagementGrants, 0, NodeTypeFolder), a.Principal))
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
		_, ok := a.Navs[navID]
		return ok && (a.Global || a.matchesAnyManagementGrant(a.managementGrantConstraints(nil, navID, 1, nodeType, navID)))
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
	if parent, ok := a.Nodes[parentID]; !ok || parent.Type != NodeTypeFolder || !a.CanManageNode(parentID) {
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
	return a.matchesAnyManagementGrant(a.managementGrantConstraints(current, parentID, distance, node.Type, navID))
}

func (a *DocumentManagementAccess) matchesAnyManagementGrant(groups [][]DocumentManagementGrant) bool {
	for _, grants := range groups {
		if a.matchesGrants(grants, a.Principal) {
			return true
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
