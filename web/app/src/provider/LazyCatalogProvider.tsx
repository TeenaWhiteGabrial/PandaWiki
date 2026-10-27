'use client';
import { StoreContext, useStore } from '@/provider';
import type { ITreeItem } from '@/assets/type';
import { getShareNodeTree, type TreeGroup } from '@/request/NodeTree';
import { convertToTree, findParentPath } from '@/utils/tree';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useParams } from 'next/navigation';

const keyOf = (nav: string, parent = '') => `${nav}/${parent}`;
const loadedFrom = (groups: TreeGroup[]) =>
  new Set(
    groups.flatMap(g => (g.loaded_parents || []).map(p => keyOf(g.nav_id, p))),
  );
const asTree = (
  groups: TreeGroup[],
  nav: string,
  expanded: Set<string>,
  states: Record<string, 'loading' | 'error'>,
): ITreeItem[] => {
  const group = groups.find(g => g.nav_id === nav);
  if (!group) return [];
  const decorate = (nodes: ITreeItem[]): ITreeItem[] =>
    nodes.map(n => ({
      ...n,
      navId: nav,
      expanded: expanded.has(n.id),
      childrenLoading: states[keyOf(nav, n.id)] === 'loading',
      childrenError: states[keyOf(nav, n.id)] === 'error',
      children: decorate(n.children || []),
    }));
  return [
    {
      id: `nav:${nav}`,
      navId: nav,
      kind: 'nav',
      name: group.nav_name,
      type: 1,
      level: 0,
      hasChildren: group.has_children,
      expanded: expanded.has(`nav:${nav}`),
      childrenLoading: states[keyOf(nav)] === 'loading',
      childrenError: states[keyOf(nav)] === 'error',
      children: decorate(convertToTree(group.list)),
    },
  ];
};

export default function LazyCatalogProvider({
  initialGroups,
  children,
}: {
  initialGroups: TreeGroup[];
  children: React.ReactNode;
}) {
  const outer = useStore();
  const params = useParams();
  const docID = typeof params?.id === 'string' ? params.id : '';
  const initialNav =
    initialGroups.find(g => g.list.some(n => n.id === docID))?.nav_id ||
    initialGroups[0]?.nav_id;
  const [groups, setGroups] = useState(initialGroups);
  const [selectedNavId, selectNav] = useState<string | undefined>(initialNav);
  const [expanded, setExpanded] = useState(
    () =>
      new Set(
        initialGroups.flatMap(g =>
          (g.loaded_parents || []).map(p => p || `nav:${g.nav_id}`),
        ),
      ),
  );
  const [states, setStates] = useState<Record<string, 'loading' | 'error'>>({});
  const revision = useRef(initialGroups[0]?.tree_revision);
  const loaded = useRef(loadedFrom(initialGroups));
  const pending = useRef(new Map<string, Promise<void>>());
  const epoch = useRef(0);
  const identity = `${outer.kbDetail?.name || ''}:${outer.authInfo?.id || initialGroups[0]?.viewer_id || ''}:${outer.authRevision || 0}`;
  const currentIdentity = useRef(identity);
  currentIdentity.current = identity;
  const snapshotIdentity = useRef(identity);
  const snapshotViewer = groups[0]?.viewer_id;
  const scopeValid =
    snapshotIdentity.current === identity &&
    !(outer.authRevision && !outer.authInfo?.id) &&
    (!snapshotViewer ||
      !outer.authInfo?.id ||
      snapshotViewer === outer.authInfo.id);

  const merge = useCallback((next: TreeGroup[], replace = false) => {
    const changed = revision.current !== next[0]?.tree_revision;
    if (changed || replace) loaded.current.clear();
    revision.current = next[0]?.tree_revision;
    setGroups(prev =>
      next.map(g => {
        if (replace || changed) return g;
        const old = prev.find(p => p.nav_id === g.nav_id);
        const byID = new Map((old?.list || []).map(n => [n.id, n]));
        // Replacing an explicitly loaded level also removes deleted siblings.
        for (const p of g.loaded_parents || [])
          for (const n of byID.values())
            if ((n.parent_id === g.nav_id ? '' : n.parent_id || '') === p)
              byID.delete(n.id);
        for (const n of g.list) byID.set(n.id, n);
        return { ...g, list: [...byID.values()] };
      }),
    );
    for (const g of next)
      for (const p of g.loaded_parents || [])
        loaded.current.add(keyOf(g.nav_id, p));
  }, []);

  const loadCatalogChildren = useCallback(
    (item: ITreeItem) => {
      const nav = item.navId || selectedNavId;
      if (!nav || !scopeValid) return Promise.resolve();
      const parent = item.kind === 'nav' ? '' : item.id;
      const key = keyOf(nav, parent);
      if (loaded.current.has(key)) return Promise.resolve();
      if (pending.current.has(key)) return pending.current.get(key)!;
      const version = epoch.current;
      const scope = identity;
      setStates(prev => ({ ...prev, [key]: 'loading' }));
      const task = getShareNodeTree({ nav_id: nav, parent_id: parent })
        .then(async next => {
          if (epoch.current !== version || currentIdentity.current !== scope)
            return;
          if (revision.current !== next?.[0]?.tree_revision) {
            next = await getShareNodeTree({
              nav_id: nav,
              parent_id: parent,
              path_id: docID || undefined,
            });
            if (epoch.current !== version || currentIdentity.current !== scope)
              return;
            merge(next || [], true);
          } else merge(next || []);
          setStates(prev => {
            const n = { ...prev };
            delete n[key];
            return n;
          });
        })
        .catch(() => {
          if (epoch.current === version && currentIdentity.current === scope)
            setStates(prev => ({ ...prev, [key]: 'error' }));
        })
        .finally(() => {
          if (pending.current.get(key) === task) pending.current.delete(key);
        });
      pending.current.set(key, task);
      return task;
    },
    [selectedNavId, merge, identity, scopeValid, docID],
  );

  useEffect(() => {
    epoch.current++;
    pending.current.clear();
    loaded.current = loadedFrom(initialGroups);
    revision.current = initialGroups[0]?.tree_revision;
    snapshotIdentity.current = identity;
    setGroups(initialGroups);
    selectNav(initialNav);
    setStates({});
    setExpanded(
      new Set(
        initialGroups.flatMap(g =>
          (g.loaded_parents || []).map(p => p || `nav:${g.nav_id}`),
        ),
      ),
    );
    return () => {
      epoch.current++;
    };
  }, [initialGroups]);

  useEffect(() => {
    if (scopeValid) return;
    const version = ++epoch.current;
    const scope = identity;
    pending.current.clear();
    loaded.current.clear();
    setGroups([]);
    setExpanded(new Set());
    setStates({});
    if (!outer.authInfo?.id) return;
    let disposed = false;
    getShareNodeTree({ path_id: docID || undefined })
      .then(next => {
        if (
          disposed ||
          version !== epoch.current ||
          currentIdentity.current !== scope
        )
          return;
        snapshotIdentity.current = scope;
        merge(next || [], true);
      })
      .catch(() => undefined);
    return () => {
      disposed = true;
    };
  }, [identity, scopeValid, outer.authInfo?.id, docID, merge]);

  useEffect(() => {
    if (!docID || !scopeValid) return;
    const group = groups.find(g => g.list.some(n => n.id === docID));
    if (group) {
      selectNav(group.nav_id);
      const path = findParentPath(convertToTree(group.list), docID) || [];
      const node = group.list.find(n => n.id === docID)!;
      setExpanded(prev => new Set([...prev, `nav:${group.nav_id}`, ...path]));
      if (node.type === 1) {
        setExpanded(prev => new Set([...prev, node.id]));
        void loadCatalogChildren({ ...node, navId: group.nav_id, level: 0 });
      }
      return;
    }
    let disposed = false;
    const version = epoch.current;
    const scope = identity;
    getShareNodeTree({ path_id: docID })
      .then(next => {
        if (
          !disposed &&
          epoch.current === version &&
          currentIdentity.current === scope
        )
          merge(next || []);
      })
      .catch(() => undefined);
    return () => {
      disposed = true;
    };
    // The branch cache handles deduplication; changes to unrelated loaded levels must not reopen collapsed folders.
  }, [
    docID,
    scopeValid,
    identity,
    groups.some(g => g.list.some(n => n.id === docID)),
  ]);

  useEffect(() => {
    let disposed = false;
    const revalidate = async () => {
      if (
        document.visibilityState !== 'visible' ||
        !scopeValid ||
        !outer.authInfo?.id
      )
        return;
      const version = epoch.current;
      const scope = identity;
      try {
        const roots = await getShareNodeTree();
        if (
          disposed ||
          epoch.current !== version ||
          currentIdentity.current !== scope ||
          revision.current === roots?.[0]?.tree_revision
        )
          return;
        const updatedVersion = ++epoch.current;
        pending.current.clear();
        loaded.current.clear();
        setGroups([]);
        setStates({});
        const next = await getShareNodeTree({ path_id: docID || undefined });
        if (
          disposed ||
          epoch.current !== updatedVersion ||
          currentIdentity.current !== scope
        )
          return;
        merge(next || [], true);
      } catch {
        /* Authentication failures are handled by the shared client. */
      }
    };
    document.addEventListener('visibilitychange', revalidate);
    return () => {
      disposed = true;
      document.removeEventListener('visibilitychange', revalidate);
    };
  }, [docID, identity, scopeValid, outer.authInfo?.id, merge]);

  const tree = useMemo(
    () =>
      scopeValid ? asTree(groups, selectedNavId || '', expanded, states) : [],
    [groups, selectedNavId, expanded, states, scopeValid],
  );
  const toggleCatalogFolder = useCallback(
    (item: ITreeItem) => {
      setExpanded(prev => {
        const next = new Set(prev);
        if (next.has(item.id)) next.delete(item.id);
        else next.add(item.id);
        return next;
      });
      if (!item.expanded) void loadCatalogChildren(item);
    },
    [loadCatalogChildren],
  );
  const searchCatalog = useCallback(
    async (search: string) => {
      const version = epoch.current;
      const scope = identity;
      const result = await getShareNodeTree({ search });
      if (epoch.current !== version || currentIdentity.current !== scope)
        return [];
      return (result || []).flatMap(g => {
        const ids = new Set(g.list.map(n => n.id));
        ids.add(`nav:${g.nav_id}`);
        return asTree([g], g.nav_id, ids, {});
      });
    },
    [identity],
  );
  const navDataMap = useMemo(
    () => Object.fromEntries(groups.map(g => [g.nav_id, g.list])),
    [groups],
  );
  return (
    <StoreContext.Provider
      value={{
        ...outer,
        tree,
        navDataMap: scopeValid ? navDataMap : {},
        nodeList: scopeValid ? groups.flatMap(g => g.list) : [],
        navList: scopeValid
          ? groups.map(g => ({
              id: g.nav_id,
              name: g.nav_name,
              position: g.position,
              entryNodeId: g.entry_node_id,
            }))
          : [],
        selectedNavId,
        setSelectedNavId: selectNav,
        toggleCatalogFolder,
        loadCatalogChildren,
        searchCatalog,
        lazyCatalog: true,
      }}
    >
      {children}
    </StoreContext.Provider>
  );
}
