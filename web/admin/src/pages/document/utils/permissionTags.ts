import type { ITreeItem } from '@/api';
import type {
  DomainDocumentManagementGrant,
  DomainNodePermissions,
} from '@/request/types';

export type PermissionTagColor =
  | 'primary'
  | 'secondary'
  | 'success'
  | 'warning'
  | 'error'
  | 'info';

export interface DocumentPermissionTag {
  label: string;
  color: PermissionTagColor;
  title?: string;
}

export interface PermissionAudienceCatalog {
  departments: Record<string, string>;
  levels: Record<string, string>;
  users: Record<string, string>;
}

const fullNames = (
  ids: string[] | undefined,
  catalog: Record<string, string>,
  fallback: string,
) => {
  const names = (ids || []).map(id => catalog[id]).filter(Boolean);
  return names.length ? names.join('、') : fallback;
};

const hasIDs = (ids: string[] | undefined) => !!ids?.length;

interface ReadingRuleClause {
  label: string;
  permissions: DomainNodePermissions;
}

function hasReadingRule(permissions: DomainNodePermissions): boolean {
  return (
    permissions.scope_type === 'department' ||
    permissions.scope_type === 'permission_level' ||
    permissions.scope_type === 'user' ||
    !!permissions.allow_all_users ||
    hasIDs(permissions.allowed_department_ids) ||
    hasIDs(permissions.allowed_permission_level_ids) ||
    hasIDs(permissions.allowed_user_ids) ||
    !!permissions.min_permission_level_id ||
    !!permissions.min_permission_level_rank ||
    hasIDs(permissions.denied_department_ids) ||
    hasIDs(permissions.denied_user_ids)
  );
}

function collectReadingRules(
  permissions: DomainNodePermissions,
  label = '本级规则',
): ReadingRuleClause[] {
  const clauses: ReadingRuleClause[] = [];
  const { constraints, ...ownRule } = permissions;
  if (hasReadingRule(ownRule)) clauses.push({ label, permissions: ownRule });
  constraints?.forEach((constraint, index) => {
    clauses.push(...collectReadingRules(constraint, `上级规则 ${index + 1}`));
  });
  return clauses;
}

function describeReadingRule(
  permissions: DomainNodePermissions,
  catalog: PermissionAudienceCatalog,
): { label: string; detail: string; color: PermissionTagColor } {
  const scope = permissions.scope_type || 'public';
  const departments = permissions.allowed_department_ids || [];
  const levels = permissions.allowed_permission_level_ids || [];
  const users = permissions.allowed_user_ids || [];
  const departmentAudience =
    (scope === 'department' && hasIDs(departments)) ||
    (scope !== 'permission_level' && scope !== 'user' && hasIDs(departments));
  const levelAudience =
    (scope === 'permission_level' && hasIDs(levels)) ||
    (scope !== 'department' && scope !== 'user' && hasIDs(levels));
  const userAudience = hasIDs(users);
  const allUsersAudience = !!permissions.allow_all_users;
  const minimumLevelName =
    catalog.levels[permissions.min_permission_level_id || ''] ||
    (permissions.min_permission_level_rank
      ? `等级值 ${permissions.min_permission_level_rank}`
      : '指定等级');
  const hasMinimum =
    !!permissions.min_permission_level_id ||
    !!permissions.min_permission_level_rank;
  const deniedDepartments = permissions.denied_department_ids || [];
  const deniedUsers = permissions.denied_user_ids || [];

  const audienceConditions: string[] = [];
  if (departmentAudience) {
    audienceConditions.push(
      `部门（${fullNames(departments, catalog.departments, '未指定部门')}）`,
    );
  }
  if (levelAudience) {
    audienceConditions.push(
      `等级（${fullNames(levels, catalog.levels, '未指定等级')}）`,
    );
  }
  if (
    hasMinimum &&
    scope !== 'user' &&
    (departmentAudience ||
      levelAudience ||
      (scope !== 'department' && scope !== 'permission_level'))
  ) {
    audienceConditions.push(`达到${minimumLevelName}及以上`);
  }
  const groupRoute = audienceConditions.length
    ? `授权对象需同时满足：${audienceConditions.join('，')}`
    : '';
  const allUsersRoute = allUsersAudience
    ? '所有已登录且符合上级规则的用户可查阅'
    : '';
  const userRoute = userAudience
    ? `指定用户白名单：${fullNames(users, catalog.users, '未指定用户')}（可单独查阅）`
    : '';
  const denied = [
    deniedDepartments.length > 0 &&
      `排除部门：${fullNames(deniedDepartments, catalog.departments, '指定部门')}`,
    deniedUsers.length > 0 &&
      `排除用户：${fullNames(deniedUsers, catalog.users, '指定用户')}`,
  ]
    .filter(Boolean)
    .join('；');
  const grantRoutes = [allUsersRoute, groupRoute, userRoute].filter(Boolean);
  const grantDetail =
    grantRoutes.length > 1
      ? `以下任一方式可授权：${grantRoutes.join('；')}`
      : grantRoutes[0] || '';
  const detailParts = [grantDetail, denied && `拒绝规则：${denied}`]
    .filter(Boolean)
    .join('；');
  const explicitLegacyScope =
    (scope === 'department' && !hasIDs(departments)) ||
    (scope === 'permission_level' && !hasIDs(levels)) ||
    (scope === 'user' && !hasIDs(users));

  if (!detailParts && explicitLegacyScope) {
    return {
      label: '查阅对象未配置',
      detail: '当前规则指定了授权方式，但没有配置对应的部门、等级或用户。',
      color: 'error',
    };
  }
  if (!detailParts) {
    return {
      label: '全员可查阅（需登录）',
      detail:
        '所有已登录用户可查阅，不包括未登录游客；本级没有额外限制查阅对象。',
      color: 'primary',
    };
  }

  const hasDenied = deniedDepartments.length > 0 || deniedUsers.length > 0;
  let label = '查阅范围受限';
  if (allUsersAudience) {
    label = '全员可查阅（需登录）';
  } else if (departmentAudience && (levelAudience || hasMinimum)) {
    label = '部门×等级限制';
  } else if (departmentAudience) {
    label = '仅限部门';
  } else if (levelAudience && hasMinimum) {
    label = '等级范围限制';
  } else if (levelAudience) {
    label = '仅限等级';
  } else if (hasMinimum) {
    label = '最低等级限制';
  } else if (userAudience) {
    label = '指定用户';
  }
  if (userAudience && (groupRoute || hasDenied)) label += '·含白名单';
  if (hasDenied) label += '·含排除规则';
  return {
    label,
    detail: detailParts,
    color: hasDenied
      ? 'error'
      : userAudience && !groupRoute
        ? 'success'
        : levelAudience && !departmentAudience
          ? 'secondary'
          : 'warning',
  };
}

function getReadingPermissionTag(
  permissions: DomainNodePermissions,
  catalog: PermissionAudienceCatalog,
): DocumentPermissionTag {
  const clauses = collectReadingRules(permissions);
  if (!clauses.length) {
    return {
      label: '全员可查阅（需登录）',
      color: 'primary',
      title:
        '所有已登录用户可查阅，不包括未登录游客；当前及上级规则均未额外限制查阅对象。',
    };
  }
  const descriptions = clauses.map(clause => ({
    clauseLabel: clause.label,
    ...describeReadingRule(clause.permissions, catalog),
  }));
  const tagLabel = clauses.length > 1 ? '多层查阅限制' : descriptions[0].label;
  const detail = descriptions
    .map(item => `${item.clauseLabel}：${item.detail}`)
    .join('；');
  return {
    label: tagLabel,
    color: clauses.length > 1 ? 'warning' : descriptions[0].color,
    title: clauses.length > 1 ? `各层查阅规则需要同时满足。${detail}` : detail,
  };
}

function directoryScopeText(
  scope: DomainDocumentManagementGrant['directory_scope'],
) {
  if (scope === 'current') return '当前目录';
  if (scope === 'current_and_descendants') return '当前及下级目录';
  return '当前及下级目录（默认）';
}

function getManagementPermissionTag(
  permissions: DomainNodePermissions,
  catalog: PermissionAudienceCatalog,
  inheritedConstraints?: DomainDocumentManagementGrant[][],
): DocumentPermissionTag {
  const groups = (
    inheritedConstraints?.length
      ? inheritedConstraints
      : [permissions.management_grants || []]
  ).filter(group => !!group?.length);
  if (!groups.length) {
    return {
      label: '仅超级管理员可管理',
      color: 'info',
      title: '当前节点没有额外管理授权；超级管理员始终拥有全局管理权限。',
    };
  }

  const grants = groups.flat();
  const subjectTypes = new Set(grants.map(grant => grant.subject_type));
  const scopes = [...new Set(grants.map(grant => grant.directory_scope))];
  const scopeText =
    scopes.length === 1
      ? directoryScopeText(scopes[0])
      : scopes.length === 0
        ? '当前及下级目录（默认）'
        : '按授权规则';
  const userIDs = [
    ...new Set(
      grants
        .filter(grant => grant.subject_type === 'user')
        .flatMap(grant => grant.subject_ids),
    ),
  ];
  let subjectText: string;
  if (groups.length > 1) {
    subjectText = '多层管理授权';
  } else if (subjectTypes.size === 1 && subjectTypes.has('user')) {
    subjectText = `指定管理员 ${userIDs.length} 人`;
  } else if (subjectTypes.size === 1 && subjectTypes.has('department')) {
    subjectText = '部门管理员';
  } else if (subjectTypes.size === 1 && subjectTypes.has('permission_level')) {
    subjectText = '等级管理员';
  } else {
    subjectText = '多种对象获管理授权';
  }
  const detail = groups
    .map((group, index) => {
      const grantDetails = group.map(grant => {
        const names =
          grant.subject_type === 'department'
            ? fullNames(grant.subject_ids, catalog.departments, '指定部门')
            : grant.subject_type === 'permission_level'
              ? fullNames(grant.subject_ids, catalog.levels, '指定等级')
              : fullNames(grant.subject_ids, catalog.users, '指定管理员');
        const subjectLabel =
          grant.subject_type === 'department'
            ? `部门管理员（${names}及下属组织）`
            : grant.subject_type === 'permission_level'
              ? `等级管理员（${names}）`
              : `指定管理员（${names}）`;
        return `${subjectLabel}，范围：${directoryScopeText(grant.directory_scope)}`;
      });
      const groupRule =
        groups.length > 1
          ? `管理授权组 ${index + 1}（组内任一项满足即可）`
          : '管理授权（任一项满足即可）';
      return `${groupRule}：${grantDetails.join('；')}`;
    })
    .join('；');
  const groupConstraint = groups.length > 1 ? '不同授权组都必须满足。' : '';
  return {
    label: `${subjectText}·${scopeText}`,
    color: 'success',
    title: `超级管理员始终拥有全局管理权限。${groupConstraint}${detail}`,
  };
}

export function getDocumentPermissionTags(
  permissions: DomainNodePermissions | undefined,
  catalog: PermissionAudienceCatalog,
  managementConstraints?: DomainDocumentManagementGrant[][],
): DocumentPermissionTag[] {
  const effectivePermissions = permissions || {};
  return [
    getReadingPermissionTag(effectivePermissions, catalog),
    getManagementPermissionTag(
      effectivePermissions,
      catalog,
      managementConstraints,
    ),
  ];
}

export function addDocumentPermissionTags(
  items: ITreeItem[],
  catalog: PermissionAudienceCatalog,
): ITreeItem[] {
  return items.map(item => {
    const permissions = item.effective_permissions || item.permissions;
    const effectiveManagementGrants = item.effective_management_grants;
    const managementGrants = effectiveManagementGrants?.length
      ? effectiveManagementGrants
      : item.permissions?.management_grants;
    const permissionSummary = permissions
      ? {
          ...permissions,
          management_grants: managementGrants,
        }
      : undefined;
    const tags = getDocumentPermissionTags(
      permissionSummary,
      catalog,
      item.effective_management_constraints,
    );
    return {
      ...item,
      permission_tags: tags,
      children: item.children
        ? addDocumentPermissionTags(item.children, catalog)
        : item.children,
    };
  });
}
