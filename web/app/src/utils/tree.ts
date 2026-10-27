import { ITreeItem, NodeListItem } from '@/assets/type';
import type { GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp } from '@/request/types';

/** 栏目项，用于导航栏展示 */
export interface NavItem {
  id: string;
  name: string;
  position?: number;
  entryNodeId?: string;
}

/**
 * 查找目标节点在树中的父级路径（从根到目标父节点）
 */
export function findParentPath(
  nodes: ITreeItem[],
  targetId: string,
  path: string[] = [],
  visited = new Set<string>(),
): string[] | null {
  for (const node of nodes) {
    if (visited.has(node.id)) {
      continue;
    }
    if (node.id === targetId) {
      return path;
    }
    if (node.children && node.children.length > 0) {
      const nextVisited = new Set(visited);
      nextVisited.add(node.id);
      const found = findParentPath(
        node.children,
        targetId,
        [...path, node.id],
        nextVisited,
      );
      if (found) return found;
    }
  }
  return null;
}

/**
 * 解析节点列表 API 返回数据
 * 支持两种格式：1) 分组格式 [{nav_id, nav_name, list, position}] 2) 扁平格式 NodeListItem[]
 */
export function parseNodeListResponse(
  data: any,
  nodeId?: string,
): {
  isGrouped: boolean;
  navList: NavItem[];
  navDataMap: Record<string, NodeListItem[]>;
  defaultNavId?: string;
} {
  if (!data || !Array.isArray(data)) {
    return { isGrouped: false, navList: [], navDataMap: {} };
  }

  const first = data[0];
  const isGrouped =
    first &&
    typeof first === 'object' &&
    'nav_id' in first &&
    'list' in first &&
    Array.isArray(first.list);

  if (isGrouped) {
    const navList: NavItem[] = [];
    const navDataMap: Record<string, NodeListItem[]> = {};
    (data as GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp[])
      .sort((a, b) => (a.position ?? 0) - (b.position ?? 0))
      .forEach(item => {
        const list = (item.list ?? []) as NodeListItem[];
        if (item.nav_id && list.length > 0) {
          navList.push({
            id: item.nav_id,
            name: item.nav_name ?? '',
            position: item.position,
          });
          navDataMap[item.nav_id] = list;
        }
      });
    const defaultNavId =
      (nodeId && findNavIdByNodeId(navDataMap, nodeId)) ?? navList[0]?.id;
    return { isGrouped: true, navList, navDataMap, defaultNavId };
  }

  return {
    isGrouped: false,
    navList: [],
    navDataMap: { '': data as NodeListItem[] },
    defaultNavId: '',
  };
}

/** 根据文档 id 在 navDataMap 中查找所属的 nav_id */
export function findNavIdByNodeId(
  navDataMap: Record<string, NodeListItem[]>,
  nodeId: string,
): string | undefined {
  if (!nodeId || !navDataMap) return undefined;
  for (const [navId, list] of Object.entries(navDataMap)) {
    if (list?.some(item => item.id === nodeId)) {
      return navId;
    }
  }
  return undefined;
}

export function convertToTree(data: NodeListItem[]) {
  const nodeMap = new Map<string, ITreeItem>();
  const rootNodes: ITreeItem[] = [];

  data.forEach(item => {
    const node: ITreeItem = {
      id: item.id,
      summary: item.summary,
      name: item.name,
      level: 0,
      status: item.status,
      order: item.position,
      emoji: item.emoji,
      pathOnly: item.path_only,
      hasChildren: item.has_children,
      type: item.type,
      parentId: item.parent_id || null,
      children: [],
      canHaveChildren: item.type === 1,
      updated_at: item.updated_at || item.created_at,
    };
    nodeMap.set(item.id, node);
  });

  const createsCycle = (nodeId: string, parentId: string) => {
    let currentParentId: string | null = parentId;
    const visited = new Set<string>([nodeId]);
    while (currentParentId) {
      if (visited.has(currentParentId)) {
        return true;
      }
      visited.add(currentParentId);
      const currentParent = nodeMap.get(currentParentId);
      currentParentId = currentParent?.parentId || null;
    }
    return false;
  };

  nodeMap.forEach(node => {
    if (
      node.parentId &&
      nodeMap.has(node.parentId) &&
      !createsCycle(node.id, node.parentId)
    ) {
      const parent = nodeMap.get(node.parentId)!;
      parent.children!.push(node);
    } else {
      rootNodes.push(node);
    }
  });

  const calculateLevel = (
    nodes: ITreeItem[],
    level: number = 0,
    path = new Set<string>(),
  ) => {
    nodes.forEach(node => {
      if (path.has(node.id)) {
        return;
      }
      node.level = level;
      if (node.children?.length) {
        const nextPath = new Set(path);
        nextPath.add(node.id);
        calculateLevel(node.children, level + 1, nextPath);
      }
    });
  };
  calculateLevel(rootNodes);

  const sortChildren = (nodes: ITreeItem[], path = new Set<string>()) => {
    nodes.sort((a, b) => (a.order ?? 0) - (b.order ?? 0));
    nodes.forEach(node => {
      if (path.has(node.id)) {
        return;
      }
      if (node.children?.length) {
        const nextPath = new Set(path);
        nextPath.add(node.id);
        sortChildren(node.children, nextPath);
      }
    });
  };
  sortChildren(rootNodes);
  return rootNodes;
}

export const filterEmptyFolders = (
  data: ITreeItem[],
  path = new Set<string>(),
): ITreeItem[] => {
  return data
    .map(item => {
      if (path.has(item.id)) {
        return { ...item, children: [] };
      }
      if (item.children && item.children.length > 0) {
        const nextPath = new Set(path);
        nextPath.add(item.id);
        const filteredChildren = filterEmptyFolders(item.children, nextPath);
        return { ...item, children: filteredChildren };
      }
      return item;
    })
    .filter(item => {
      if (item.type === 1) {
        return !!item.hasChildren || !!item.children?.length;
      }
      return true;
    });
};

export const addExpandState = (
  nodes: ITreeItem[],
  activeId: string,
  defaultExpand: boolean,
): { tree: ITreeItem[] } => {
  const parentPath = findParentPath(nodes, activeId) || [];
  const parentSet = new Set(parentPath);

  const addExpand = (
    nodes: ITreeItem[],
    path = new Set<string>(),
  ): ITreeItem[] => {
    return nodes.map(node => {
      if (path.has(node.id)) {
        return { ...node, children: [] };
      }
      const isExpanded =
        node.expanded ?? (parentSet.has(node.id) ? true : defaultExpand);
      if (node.children && node.children.length > 0) {
        const nextPath = new Set(path);
        nextPath.add(node.id);
        return {
          ...node,
          defaultExpand: isExpanded,
          expanded: isExpanded,
          children: addExpand(node.children, nextPath),
        };
      }
      return node;
    });
  };
  return { tree: addExpand(nodes) };
};
