package domain

import "slices"

const (
	ManagementSubjectLevel      = "permission_level"
	ManagementSubjectDepartment = "department"
	ManagementSubjectUser       = "user"
	ManagementScopeTree         = "current_and_descendants"
	ManagementScopeCurrent      = "current"
)

// A grant group is a union of subjects at one hierarchy level. A manager must
// match every applicable group from the node through its ancestors.
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
	allowed := a.matchesAllManagementConstraints(a.ManagementGrantConstraintsForNode(id))
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

// ManagementGrantConstraintsForNode returns the applicable grant groups from
// this node, its ancestors, and its navigation root. Grants within each group
// are alternatives; every returned group must be satisfied.
func (a *DocumentManagementAccess) ManagementGrantConstraintsForNode(id string) [][]DocumentManagementGrant {
	return a.managementGrantConstraintsForNode(id, map[string]bool{})
}

func (a *DocumentManagementAccess) managementGrantConstraintsForNode(id string, visiting map[string]bool) (constraints [][]DocumentManagementGrant) {
	if a == nil {
		return nil
	}
	node, exists := a.Nodes[id]
	if !exists || visiting[id] {
		return nil
	}
	if a.evaluation != nil {
		if cached, found := a.evaluation.constraints[id]; found {
			return slices.Clone(cached)
		}
		defer func() { a.evaluation.constraints[id] = slices.Clone(constraints) }()
	}
	visiting[id] = true
	defer delete(visiting, id)

	// Keep directory-scope rules: a current-directory grant does not itself
	// authorize a nested folder. If any rule authorizes this folder, also require
	// the principal to manage every parent folder, even if a direct child grant
	// would otherwise match.
	constraints = a.managementGrantConstraints(node, node.ParentID, 0, node.Type)
	if len(constraints) == 0 || node.Type != NodeTypeFolder || node.ParentID == "" || node.ParentID == node.NavId {
		return constraints
	}
	parent, ok := a.Nodes[node.ParentID]
	if !ok || parent.Type != NodeTypeFolder || parent.NavId != node.NavId {
		return nil
	}
	parentConstraints := a.managementGrantConstraintsForNode(parent.ID, visiting)
	if len(parentConstraints) == 0 {
		return nil
	}
	return appendUniqueManagementConstraints(parentConstraints, constraints)
}

func appendUniqueManagementConstraints(result, additions [][]DocumentManagementGrant) [][]DocumentManagementGrant {
	for _, addition := range additions {
		duplicate := false
		for _, existing := range result {
			if len(existing) != len(addition) {
				continue
			}
			duplicate = true
			for index, grant := range existing {
				other := addition[index]
				if grant.SubjectType != other.SubjectType || grant.DirectoryScope != other.DirectoryScope || !slices.Equal(grant.SubjectIDs, other.SubjectIDs) {
					duplicate = false
					break
				}
			}
			if duplicate {
				break
			}
		}
		if !duplicate {
			result = append(result, addition)
		}
	}
	return result
}

// ManagementGrantConstraintsForParentNode returns only the constraints
// inherited by a node from its parent folders. Top-level nodes have no parent
// folder and can delegate to any administrator.
func (a *DocumentManagementAccess) ManagementGrantConstraintsForParentNode(id string) [][]DocumentManagementGrant {
	if a == nil {
		return nil
	}
	node, exists := a.Nodes[id]
	if !exists {
		return nil
	}
	if node.ParentID == "" || node.ParentID == node.NavId {
		return nil
	}
	parent := a.Nodes[node.ParentID]
	if parent == nil || parent.Type != NodeTypeFolder || parent.NavId != node.NavId {
		return [][]DocumentManagementGrant{{}}
	}
	return a.managementGrantConstraintsForNode(parent.ID, map[string]bool{})
}

func (a *DocumentManagementAccess) managementGrantConstraints(current *Node, parentID string, distance int, targetType NodeType, navIDOverride ...string) [][]DocumentManagementGrant {
	if a == nil {
		return nil
	}
	navID := ""
	if current != nil {
		navID = current.NavId
	}
	if len(navIDOverride) > 0 && navIDOverride[0] != "" {
		navID = navIDOverride[0]
	}
	visited := map[string]bool{}
	var constraints [][]DocumentManagementGrant
	for current != nil && !visited[current.ID] {
		visited[current.ID] = true
		if current.NavId != navID {
			return constraints
		}
		applicable := applicableManagementGrants(current.Permissions.ManagementGrants, distance, targetType)
		if len(applicable) > 0 {
			constraints = append(constraints, applicable)
		}
		// A top-level folder with an applicable local rule starts the folder
		// hierarchy, so its scope defines managers below it. Navigation grants
		// are a fallback when no local folder rule applies at this distance.
		if current.Type == NodeTypeFolder && (current.ParentID == "" || current.ParentID == current.NavId) && len(applicable) > 0 {
			return constraints
		}
		parentID = current.ParentID
		current = a.Nodes[parentID]
		distance++
	}
	if current == nil && (parentID == "" || parentID == navID) {
		if nav := a.Navs[navID]; nav != nil {
			if applicable := applicableManagementGrants(nav.Permissions.ManagementGrants, distance, targetType); len(applicable) > 0 {
				constraints = append(constraints, applicable)
			}
		}
	}
	return constraints
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
		return ok && (a.Global || a.matchesAllManagementConstraints(a.managementGrantConstraints(nil, navID, 1, nodeType, navID)))
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
	return a.matchesAllManagementConstraints(a.managementGrantConstraints(current, parentID, distance, node.Type, navID))
}

func (a *DocumentManagementAccess) matchesAllManagementConstraints(constraints [][]DocumentManagementGrant) bool {
	if len(constraints) == 0 {
		return false
	}
	for _, grants := range constraints {
		if !a.matchesGrants(grants, a.Principal) {
			return false
		}
	}
	return true
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
