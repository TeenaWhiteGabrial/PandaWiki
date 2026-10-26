import type { DomainNodePermissions } from '@/request/types';

interface LevelOption {
  id: string;
  rank: number;
  status?: string;
}

interface DepartmentOption {
  id: string;
  parent_id?: string | null;
  path?: string;
  ancestor_ids?: string[];
  status?: string;
}

// Department choices follow the configured organization scope. Grade rules,
// excluded individuals and current membership must not exclude a whole
// department; those rules are intersected when actual read access is evaluated.
export function selectableReadingDepartments<T extends DepartmentOption>(
  departments: T[],
  parents: DomainNodePermissions[],
): T[] {
  const departmentsByID = new Map(departments.map(item => [item.id, item]));
  const isWithin = (department: DepartmentOption, ancestorID: string) => {
    if (department.id === ancestorID) return true;
    if (department.ancestor_ids?.includes(ancestorID)) return true;
    const seen = new Set<string>();
    let parentID = department.parent_id;
    while (parentID && !seen.has(parentID)) {
      if (parentID === ancestorID) return true;
      seen.add(parentID);
      parentID = departmentsByID.get(parentID)?.parent_id;
    }
    const ancestorPath = departmentsByID.get(ancestorID)?.path;
    return (
      !!ancestorPath &&
      !!department.path &&
      (department.path === ancestorPath ||
        department.path.startsWith(`${ancestorPath}/`))
    );
  };
  const permitsDepartment = (
    permission: DomainNodePermissions,
    department: DepartmentOption,
  ): boolean => {
    if (
      !(permission.constraints || []).every(rule =>
        permitsDepartment(rule, department),
      )
    ) {
      return false;
    }
    if (
      permission.denied_department_ids?.some(id => isWithin(department, id))
    ) {
      return false;
    }
    if (permission.allow_all_users) {
      return true;
    }
    // Personal exceptions do not grant the recipient's entire department.
    // They remain available through the separate user selector.
    if (permission.scope_type === 'user') return false;
    if (permission.scope_type === 'permission_level') {
      return true;
    }
    const allowed = permission.allowed_department_ids || [];
    if (allowed.length) {
      return allowed.some(id => isWithin(department, id));
    }
    if (permission.scope_type === 'department') return false;
    const personalOnly =
      !!permission.allowed_user_ids?.length &&
      !permission.allowed_permission_level_ids?.length &&
      !permission.min_permission_level_id &&
      !permission.min_permission_level_rank;
    return !personalOnly;
  };
  return departments.filter(
    department =>
      (!department.status || department.status === 'active') &&
      parents.every(parent => permitsDepartment(parent, department)),
  );
}

// Options describe permission rules, not today's user assignments. An unused
// level remains a valid restriction, including for users assigned in the future.
export function selectableReadingLevels<T extends LevelOption>(
  levels: T[],
  parents: DomainNodePermissions[],
  levelsDisabled = false,
): T[] {
  const permitsLevel = (
    permission: DomainNodePermissions,
    level: LevelOption,
  ): boolean => {
    if (
      !(permission.constraints || []).every(rule => permitsLevel(rule, level))
    ) {
      return false;
    }
    if (levelsDisabled || permission.allow_all_users) return true;
    // Personal grants are independent of local grade rules and can apply to a
    // user of any grade. The server still intersects all actual parent rules.
    if (permission.allowed_user_ids?.length) return true;
    if (permission.scope_type === 'user') return true;

    const minimumRank = permission.min_permission_level_id
      ? (levels.find(item => item.id === permission.min_permission_level_id)
          ?.rank ??
        permission.min_permission_level_rank ??
        0)
      : permission.min_permission_level_rank || 0;
    if (level.rank < minimumRank) return false;
    if (permission.scope_type === 'department') return true;
    const selected = permission.allowed_permission_level_ids || [];
    if (permission.scope_type === 'permission_level' || selected.length) {
      return selected.includes(level.id);
    }
    return true;
  };
  return levels.filter(
    level =>
      (!level.status || level.status === 'active') &&
      parents.every(parent => permitsLevel(parent, level)),
  );
}
