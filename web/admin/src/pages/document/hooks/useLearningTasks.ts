import { useEffect, useRef, useState } from 'react';
import {
  getLearningTasks,
  getLearningTaskSummary,
  type LearningTaskCounts,
  type LearningTaskPage,
  type LearningTaskQuery,
} from '@/request/nodeLearning';

// Cache lives only in the mounted panel, is bounded, and is isolated by both
// identity and KB. Never persist task data to localStorage or share across users.
export function useLearningTasks(
  open: boolean,
  identity: string,
  query: LearningTaskQuery,
) {
  const scope = `${identity}:${query.kb_id}`;
  const key = JSON.stringify([
    scope,
    query.filter,
    query.keyword,
    query.page,
    query.page_size,
    query.only_running,
  ]);
  const cache = useRef({
    scope,
    pages: new Map<string, LearningTaskPage>(),
    counts: undefined as LearningTaskCounts | undefined,
    checkedAt: 0,
  });
  if (cache.current.scope !== scope) {
    cache.current = {
      scope,
      pages: new Map(),
      counts: undefined,
      checkedAt: 0,
    };
  }
  const [, notify] = useState(0);
  const [failure, setFailure] = useState<{
    key: string;
    list: boolean;
    summary: boolean;
  }>();
  const [fetchingKey, setFetchingKey] = useState('');
  const [revision, setRevision] = useState(0);
  const forceSummary = useRef(false);
  const data = cache.current.pages.get(key);
  const counts = cache.current.counts;
  const error = failure?.key === key && failure.list;
  const summaryError = failure?.key === key && failure.summary;

  useEffect(() => {
    if (!open || !query.kb_id || !identity) return;
    const controller = new AbortController();
    let stopped = false;
    let timer: ReturnType<typeof setTimeout>;
    let inFlight = false;
    let latestCounts = cache.current.counts;
    let latestPageRunning = 0;
    const load = async () => {
      if (stopped || inFlight) return;
      clearTimeout(timer);
      if (document.visibilityState === 'hidden') {
        timer = setTimeout(load, 5000);
        return;
      }
      inFlight = true;
      setFetchingKey(key);
      setFailure(undefined);
      // Counts must never hold up rendering a successful page request.
      let listFailed = false;
      let countsFailed = false;
      let denied = false;
      const recordDenial = (error: { status?: number }) => {
        if (error?.status === 401 || error?.status === 403) denied = true;
      };
      const refreshCounts =
        forceSummary.current ||
        !cache.current.counts ||
        Date.now() - cache.current.checkedAt >= 5000;
      forceSummary.current = false;
      await Promise.all([
        getLearningTasks(query, controller.signal)
          .then(result => {
            if (stopped || denied || cache.current.scope !== scope) return;
            cache.current.pages.delete(key);
            cache.current.pages.set(key, result);
            if (cache.current.pages.size > 20) {
              cache.current.pages.delete(
                cache.current.pages.keys().next().value!,
              );
            }
            latestPageRunning = result.running;
            notify(value => value + 1);
          })
          .catch(error => {
            recordDenial(error);
            listFailed = true;
          }),
        refreshCounts
          ? getLearningTaskSummary(query.kb_id, controller.signal)
              .then(result => {
                if (stopped || denied || cache.current.scope !== scope) return;
                latestCounts = result;
                cache.current.counts = result;
                cache.current.checkedAt = Date.now();
                notify(value => value + 1);
              })
              .catch(error => {
                recordDenial(error);
                countsFailed = true;
              })
          : Promise.resolve(),
      ]);
      if (stopped || cache.current.scope !== scope) return;
      if (denied) {
        cache.current.pages.clear();
        cache.current.counts = undefined;
        cache.current.checkedAt = 0;
        notify(value => value + 1);
        listFailed = countsFailed = true;
      }
      setFailure({ key, list: listFailed, summary: countsFailed });
      setFetchingKey('');
      inFlight = false;
      // One non-overlapping refresh loop. Quiet panels use a slower interval;
      // switching back from a hidden tab revalidates immediately.
      timer = setTimeout(
        load,
        latestCounts?.running || latestPageRunning ? 5000 : 30000,
      );
    };
    const onVisible = () => {
      if (document.visibilityState === 'visible') void load();
    };
    document.addEventListener('visibilitychange', onVisible);
    void load();
    return () => {
      stopped = true;
      controller.abort();
      clearTimeout(timer);
      document.removeEventListener('visibilitychange', onVisible);
    };
    // key contains every query field; revision is manual refresh / mutation.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open, key, revision]);

  return {
    data,
    counts,
    initialLoading: !data && !error,
    loadError: error ? (data ? 'refresh' : 'initial') : null,
    summaryError,
    fetching: fetchingKey === key,
    reload: () => {
      forceSummary.current = true;
      setRevision(value => value + 1);
    },
    invalidate: () => {
      cache.current.pages.clear();
      forceSummary.current = true;
      setRevision(value => value + 1);
    },
  };
}
