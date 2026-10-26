import { getApiV1User, getApiV1UserList, type UserListQuery } from './User';
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

export const getApiV1UserListCompat = async (query: UserListQuery = {}) => {
  const resp = await getApiV1UserList(query);
  const users = ((resp.users || []) as UserListItemCompat[]) ?? [];
  return {
    users,
    total: resp.total ?? users.length,
    page: resp.page ?? query.page ?? 1,
    page_size: resp.page_size ?? query.page_size ?? 20,
  };
};
