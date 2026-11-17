import type { DomainNodePermissions, DomainReadingRule } from '@/request/types';

const clone = <T>(value: T): T => JSON.parse(JSON.stringify(value));
export const hasLocalReadingPolicy = (p: DomainNodePermissions): boolean =>
  p.reading_version === 2 ||
  !!p.scope_type ||
  !!p.allow_all_users ||
  !!p.allowed_department_ids?.length ||
  !!p.allowed_permission_level_ids?.length ||
  !!p.allowed_user_ids?.length ||
  !!p.denied_user_ids?.length ||
  !!p.denied_department_ids?.length ||
  !!p.min_permission_level_id ||
  !!p.min_permission_level_rank ||
  !!p.constraints?.length;

// The audience endpoint may wrap one policy in an unrestricted AND envelope.
export function unwrapReadingPolicy(
  p: DomainNodePermissions,
): DomainNodePermissions {
  if (
    !hasLocalReadingPolicy({ ...p, constraints: undefined }) &&
    p.constraints?.length === 1
  ) {
    return unwrapReadingPolicy(p.constraints[0]);
  }
  return p;
}

export function independentReadingPolicy(
  policy: DomainNodePermissions,
): DomainNodePermissions {
  const p = unwrapReadingPolicy(clone(policy));
  if (p.reading_version === 2 && !p.constraints?.some(hasLocalReadingPolicy)) {
    return {
      reading_version: 2,
      inherit: false,
      reading_rules: p.reading_rules || [],
      denied_user_ids: p.denied_user_ids || [],
      denied_department_ids: p.denied_department_ids || [],
    };
  }
  // Preserve the complete old intersection as one rule, rather than accidentally
  // converting each historical constraint into a separate OR grant.
  const deniedUsers = new Set<string>(),
    deniedDepartments = new Set<string>();
  const collect = (value: DomainNodePermissions): DomainNodePermissions => {
    (value.denied_user_ids || []).forEach(id => deniedUsers.add(id));
    (value.denied_department_ids || []).forEach(id =>
      deniedDepartments.add(id),
    );
    const {
      management_grants: _management,
      inherit: _inherit,
      denied_user_ids: _users,
      denied_department_ids: _departments,
      ...copy
    } = value;
    return {
      ...copy,
      ...(copy.constraints
        ? { constraints: copy.constraints.map(collect) }
        : {}),
    };
  };
  let legacy = collect(p);
  // Keep nested OR policies within an AND envelope for mixed/batch snapshots.
  if (legacy.reading_version === 2) legacy = { constraints: [legacy] };
  return {
    reading_version: 2,
    inherit: false,
    reading_rules: [
      {
        permissions: hasLocalReadingPolicy(legacy)
          ? legacy
          : { scope_type: 'public', allow_all_users: true },
        include_descendants: true,
      },
    ],
    denied_user_ids: [...deniedUsers],
    denied_department_ids: [...deniedDepartments],
  };
}

export const readingRuleKind = (
  rule: DomainReadingRule,
): 'condition' | 'users' | 'all' | 'legacy' => {
  const p = rule.permissions;
  if (
    p.constraints?.some(hasLocalReadingPolicy) ||
    p.min_permission_level_id ||
    p.min_permission_level_rank ||
    p.denied_user_ids?.length ||
    p.denied_department_ids?.length ||
    (!!p.allowed_user_ids?.length &&
      (!!p.allowed_department_ids?.length ||
        !!p.allowed_permission_level_ids?.length))
  )
    return 'legacy';
  if (
    p.allow_all_users ||
    (p.scope_type === 'public' &&
      !p.allowed_department_ids?.length &&
      !p.allowed_permission_level_ids?.length &&
      !p.allowed_user_ids?.length)
  )
    return 'all';
  if (p.scope_type === 'user' || p.allowed_user_ids?.length) return 'users';
  return 'condition';
};
export const newReadingRule = (): DomainReadingRule => ({
  permissions: { scope_type: 'custom' },
  include_descendants: true,
});
export const incompleteReadingRule = (rule: DomainReadingRule): boolean => {
  const kind = readingRuleKind(rule),
    p = rule.permissions;
  return kind === 'users'
    ? !p.allowed_user_ids?.length
    : kind === 'condition' &&
        !p.allowed_department_ids?.length &&
        !p.allowed_permission_level_ids?.length;
};

export function describeReadingPolicy(
  p: DomainNodePermissions,
  catalogs: {
    departments: Record<string, string>;
    levels: Record<string, string>;
    users: Record<string, string>;
  },
): string {
  const names = (ids: string[] | undefined, map: Record<string, string>) =>
    (ids || []).map(id => map[id] || id).join('、');
  if (p.reading_version === 2) {
    const rules = (p.reading_rules || []).map(rule =>
      describeReadingPolicy(rule.permissions, catalogs),
    );
    const deny = names(p.denied_user_ids, catalogs.users);
    const denyDept = names(p.denied_department_ids, catalogs.departments);
    const own = rules.length
      ? rules.map(text => `（${text}）`).join(' 或 ')
      : '未授权普通用户，仅超级管理员可查阅';
    return (
      [
        own,
        ...(p.constraints || [])
          .filter(hasLocalReadingPolicy)
          .map(c => describeReadingPolicy(c, catalogs)),
      ].join(' 且 ') +
      `${deny ? `；排除：${deny}` : ''}${denyDept ? `；排除部门：${denyDept}` : ''}`
    );
  }
  const conditions = [
    names(p.allowed_department_ids, catalogs.departments),
    names(p.allowed_permission_level_ids, catalogs.levels),
  ].filter(Boolean);
  const minimum = p.min_permission_level_id
    ? `达到${catalogs.levels[p.min_permission_level_id] || p.min_permission_level_id}及以上`
    : p.min_permission_level_rank
      ? `等级值不低于${p.min_permission_level_rank}`
      : '';
  if (minimum) conditions.push(minimum);
  const users = names(p.allowed_user_ids, catalogs.users);
  const group = p.scope_type === 'user' ? '' : conditions.join(' 且 ');
  const own = p.allow_all_users
    ? '全员（需登录）'
    : [group, users && `指定用户：${users}`].filter(Boolean).join(' 或 ') ||
      (p.scope_type === 'user' ||
      p.scope_type === 'department' ||
      p.scope_type === 'permission_level'
        ? '未指定对象'
        : '全员（需登录）');
  const denies = [
    names(p.denied_user_ids, catalogs.users),
    names(p.denied_department_ids, catalogs.departments),
  ]
    .filter(Boolean)
    .join('、');
  return (
    [
      own,
      ...(p.constraints || [])
        .filter(hasLocalReadingPolicy)
        .map(c => describeReadingPolicy(c, catalogs)),
    ]
      .map((text, i, all) => (all.length > 1 ? `（${text}）` : text))
      .join(' 且 ') + (denies ? `；排除：${denies}` : '')
  );
}
