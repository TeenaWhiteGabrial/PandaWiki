import { getApiV1User, getApiV1UserList } from './User';
import type { ConstsUserRole } from './types';

export type UserInfoCompat = {
  permission_levels_disabled?: boolean;
  id?: string;
  account?: string;
  name?: string;
  role?: ConstsUserRole;
  is_token?: boolean;
  last_access?: string;
  created_at?: string;
  permission_level_id?: string;
  department_ids?: string[];
};

export type UserListItemCompat = {
  id?: string;
  account?: string;
  name?: string;
  role?: ConstsUserRole;
  last_access?: string;
  created_at?: string;
  permission_level_id?: string;
  department_ids?: string[];
};

export const getApiV1UserCompat = () => getApiV1User() as Promise<UserInfoCompat>;

export const getApiV1UserListCompat = async (query?: {
  keyword?: string;
  role?: ConstsUserRole | '';
  department_id?: string;
}) => {
  const resp = await getApiV1UserList();
  const users = ((resp.users || []) as UserListItemCompat[]) ?? [];

  const filteredUsers = users.filter(user => {
    if (query?.keyword) {
      const keyword = query.keyword.toLowerCase();
      const matched =
        user.account?.toLowerCase().includes(keyword) ||
        user.name?.toLowerCase().includes(keyword);
      if (!matched) return false;
    }

    if (query?.role && user.role !== query.role) {
      return false;
    }

    if (
      query?.department_id &&
      !user.department_ids?.includes(query.department_id)
    ) {
      return false;
    }

    return true;
  });

  return {
    users: filteredUsers,
  };
};
