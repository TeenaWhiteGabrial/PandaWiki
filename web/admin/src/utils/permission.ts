import {
  ConstsUserRole,
  type DomainKnowledgeBaseDetail,
  type V1UserInfoResp,
} from '@/request/types';

export const isAdministrator = (role?: ConstsUserRole) =>
  role === ConstsUserRole.UserRoleAdmin ||
  role === ConstsUserRole.UserRoleManager;

export const isOrdinaryReadOnlyUser = (
  user?: V1UserInfoResp,
  kbDetail?: DomainKnowledgeBaseDetail,
) => !isAdministrator(user?.role);
