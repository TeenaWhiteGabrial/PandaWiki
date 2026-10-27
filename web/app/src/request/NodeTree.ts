import httpRequest, { type RequestParams } from './httpClient';
import type { NodeListItem } from '@/assets/type';
export interface TreeGroup {
  nav_id: string;
  nav_name: string;
  position: number;
  count: number;
  has_children: boolean;
  entry_node_id?: string;
  loaded_parents?: string[];
  tree_revision?: string;
  viewer_id?: string;
  list: NodeListItem[];
}
export const getShareNodeTree = (
  query: {
    nav_id?: string;
    parent_id?: string;
    path_id?: string;
    search?: string;
  } = {},
  params: RequestParams = {},
) =>
  httpRequest<{ data?: TreeGroup[] }>({
    path: '/share/v1/node/tree',
    method: 'GET',
    query,
    format: 'json',
    cache: 'no-store',
    ...params,
  });
