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
