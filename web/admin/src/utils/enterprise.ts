import type { Department } from '@/request/Enterprise';

export const buildDepartmentTree = (
  departments: Department[],
): Department[] => {
  const map = new Map<string, Department>();
  const roots: Department[] = [];

  departments.forEach(item => {
    map.set(item.id, { ...item, children: [] });
  });

  map.forEach(item => {
    if (item.parent_id && map.has(item.parent_id)) {
      map.get(item.parent_id)?.children?.push(item);
    } else {
      roots.push(item);
    }
  });

  const sortNodes = (nodes: Department[]) => {
    nodes.sort((a, b) => {
      if (a.sort !== b.sort) return a.sort - b.sort;
      return a.name.localeCompare(b.name);
    });
    nodes.forEach(node => sortNodes(node.children || []));
  };

  sortNodes(roots);
  return roots;
};

export const flattenDepartmentTree = (
  departments: Department[],
): Department[] => {
  const result: Department[] = [];
  const walk = (nodes: Department[]) => {
    nodes.forEach(node => {
      result.push(node);
      if (node.children?.length) {
        walk(node.children);
      }
    });
  };
  walk(departments);
  return result;
};

export const flattenExpandedDepartmentTree = (
  departments: Department[],
  expandedIds: Set<string>,
) => {
  const result: Department[] = [];

  const walk = (nodes: Department[]) => {
    nodes.forEach(node => {
      result.push(node);
      if (node.children?.length && expandedIds.has(node.id)) {
        walk(node.children);
      }
    });
  };

  walk(departments);
  return result;
};

export const getDepartmentDepth = (department: Department) => {
  if (!department.path) return 0;
  return Math.max(0, department.path.split('/').length - 1);
};

export const formatDepartmentOptionLabel = (department: Department) => {
  const depth = getDepartmentDepth(department);
  return `${'　'.repeat(depth)}${depth > 0 ? '└ ' : ''}${department.name}`;
};

/**
 * 获取选中节点的所有祖先部门 ID
 * 用于提交时补全祖先链
 */
export const getAncestorIds = (
  selectedIds: string[],
  departments: Department[],
): string[] => {
  const map = new Map(departments.map(d => [d.id, d]));
  const result = new Set<string>();

  selectedIds.forEach(id => {
    let current = map.get(id);
    while (current?.parent_id) {
      result.add(current.parent_id);
      current = map.get(current.parent_id);
    }
  });

  return [...result];
};

/**
 * 获取指定节点的所有后代部门 ID
 */
const getDescendantIds = (
  nodeId: string,
  departments: Department[],
): Set<string> => {
  const map = new Map<string, Department[]>();
  departments.forEach(d => {
    if (d.parent_id) {
      const children = map.get(d.parent_id) || [];
      children.push(d);
      map.set(d.parent_id, children);
    }
  });

  const result = new Set<string>();
  const walk = (parentId: string) => {
    const children = map.get(parentId) || [];
    children.forEach(child => {
      result.add(child.id);
      walk(child.id);
    });
  };
  walk(nodeId);
  return result;
};

/**
 * 获取指定节点的所有祖先部门 ID
 */
const getAncestorIdsSet = (
  nodeId: string,
  departments: Department[],
): Set<string> => {
  const map = new Map(departments.map(d => [d.id, d]));
  const result = new Set<string>();
  let current = map.get(nodeId);
  while (current?.parent_id) {
    result.add(current.parent_id);
    current = map.get(current.parent_id);
  }
  return result;
};

/**
 * 标准化选中的部门 ID 列表：
 * - 选中子节点时，自动移除其祖先和后代的选中
 * - 选中父节点时，自动移除其后代的选中
 * - 返回最精简的选中列表（不含隐含的祖先/后代）
 */
export const normalizeSelectedDepartmentIDs = (
  selectedIDs: string[],
  departments: Department[],
  preferredID?: string,
): string[] => {
  if (!selectedIDs.length) return [];

  const selectedSet = new Set(selectedIDs);

  if (preferredID && selectedSet.has(preferredID)) {
    const ancestorIds = getAncestorIdsSet(preferredID, departments);
    const descendantIds = getDescendantIds(preferredID, departments);
    ancestorIds.forEach(id => selectedSet.delete(id));
    descendantIds.forEach(id => selectedSet.delete(id));
    selectedSet.add(preferredID);
  }

  return selectedIDs.filter(id => selectedSet.has(id));
};
