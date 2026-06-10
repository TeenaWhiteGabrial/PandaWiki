import {
  ConstsUserKBPermission,
  ConstsUserRole,
  type DomainKnowledgeBaseDetail,
  type V1UserInfoResp,
} from '@/request/types';

export const isOrdinaryReadOnlyUser = (
  user?: V1UserInfoResp,
  kbDetail?: DomainKnowledgeBaseDetail,
) =>
  user?.role === ConstsUserRole.UserRoleUser &&
  kbDetail?.perm === ConstsUserKBPermission.UserKBPermissionNotNull;
