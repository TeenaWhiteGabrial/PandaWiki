import { useCallback, useEffect, useRef, useState } from 'react';
import { getDocumentTree } from '@/request/NodeTree';
import type { GithubComChaitinPandaWikiApiNodeV1NodeListGroupNavResp as Group } from '@/request/types';

export type BranchState = 'loading' | 'error';
export const branchKey = (nav: string, parent = '') => `${nav}/${parent}`;

export function useLazyDocumentTree(
  kbID: string,
  identity: string,
  search = '',
  pathID = '',
) {
  const scope = `${kbID}:${identity}:${search}`;
  const current = useRef(scope);
  current.current = scope;
  const generation = useRef(0);
  const path = useRef(pathID);
  path.current = pathID;
  const located = useRef(new Set<string>());
  const displayedScope = useRef(scope);
  const revision = useRef<string | undefined>(undefined);
  const loaded = useRef(new Map<string, string>());
  const pending = useRef(new Map<string, Promise<void>>());
  const [groups, setGroups] = useState<Group[]>([]);
  const [branches, setBranches] = useState<Record<string, BranchState>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  const refresh = useCallback(
    async (reset = false) => {
      if (!kbID) return [];
      const version = ++generation.current;
      const parents = reset
        ? []
        : [
            ...new Set([
              ...loaded.current.values(),
              ...[...pending.current.keys()].map(
                k => k.split('/')[1] || `nav:${k.split('/')[0]}`,
              ),
            ]),
          ];
      pending.current.clear();
      setLoading(true);
      setError(false);
      setBranches({});
      try {
        if (path.current) located.current.add(path.current);
        const list = await getDocumentTree({
          kb_id: kbID,
          search: search || undefined,
          path_id: path.current || undefined,
          parents,
        });
        if (current.current !== scope || version !== generation.current)
          return [];
        loaded.current.clear();
        revision.current = list?.[0]?.tree_revision;
        for (const group of list || [])
          for (const parent of group.loaded_parents || [])
            loaded.current.set(
              branchKey(group.nav_id!, parent),
              parent || `nav:${group.nav_id}`,
            );
        displayedScope.current = scope;
        setGroups(list || []);
        return list || [];
      } catch {
        if (current.current === scope && version === generation.current) {
          setError(true);
          setGroups([]);
        }
        return [];
      } finally {
        if (current.current === scope && version === generation.current)
          setLoading(false);
      }
    },
    [kbID, scope, search],
  );

  const loadChildren = useCallback(
    (nav: string, parent = ''): Promise<void> => {
      const key = branchKey(nav, parent);
      if (!kbID || search || loaded.current.has(key)) return Promise.resolve();
      const existing = pending.current.get(key);
      if (existing) return existing;
      const version = generation.current;
      setBranches(prev => ({ ...prev, [key]: 'loading' }));
      const task = getDocumentTree({
        kb_id: kbID,
        nav_id: nav,
        parent_id: parent,
      })
        .then(async result => {
          if (current.current !== scope || generation.current !== version)
            return;
          if (revision.current !== result?.[0]?.tree_revision) {
            loaded.current.set(key, parent || `nav:${nav}`);
            await refresh();
            return;
          }
          // Server revalidates the roots as well, so revoked columns disappear.
          setGroups(prev =>
            (result || []).map(group => {
              const old = prev.find(g => g.nav_id === group.nav_id);
              if (group.nav_id !== nav)
                return { ...group, list: old?.list || [] };
              const siblings = (old?.list || []).filter(
                n => (n.parent_id === nav ? '' : n.parent_id || '') !== parent,
              );
              return { ...group, list: [...siblings, ...(group.list || [])] };
            }),
          );
          loaded.current.set(key, parent || `nav:${nav}`);
          setBranches(prev => {
            const next = { ...prev };
            delete next[key];
            return next;
          });
        })
        .catch(() => {
          if (current.current === scope && generation.current === version)
            setBranches(prev => ({ ...prev, [key]: 'error' }));
        })
        .finally(() => {
          if (pending.current.get(key) === task) pending.current.delete(key);
        });
      pending.current.set(key, task);
      return task;
    },
    [kbID, scope, search, refresh],
  );

  useEffect(() => {
    loaded.current.clear();
    pending.current.clear();
    located.current.clear();
    setGroups([]);
    void refresh(true);
    return () => {
      generation.current++;
    };
  }, [scope, refresh]);

  useEffect(() => {
    if (
      pathID &&
      !loading &&
      !located.current.has(pathID) &&
      !groups.some(g => g.list?.some(n => n.id === pathID))
    )
      void refresh();
  }, [pathID, groups, loading, refresh]);
  return {
    groups: displayedScope.current === scope ? groups : [],
    setGroups,
    loading,
    error,
    branches,
    loadChildren,
    refresh,
  };
}
