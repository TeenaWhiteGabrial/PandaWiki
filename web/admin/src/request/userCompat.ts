import { getApiV1User, getApiV1UserList } from './User';
import type { ConstsUserRole } from './types';

export type UserInfoCompat = {
  id?: string;
  account?: string;
  name?: string;
  role?: ConstsUserRole;
  is_token?: boolean;
  last_access?: string;
  created_at?: string;
  permission_level_id?: string;
  department_ids?: string[];
  primary_department?: string;
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
  primary_department?: string;
};

export const getApiV1UserCompat = () => getApiV1User() as Promise<UserInfoCompat>;

export const getApiV1UserListCompat = async (query?: {
  keyword?: string;
  role?: ConstsUserRole | '';
  department_id?: string;
}) => {
  const resp = await getApiV1UserList({
    query,
  });
  return {
    users: ((resp.users || []) as UserListItemCompat[]) ?? [],
  };
};
