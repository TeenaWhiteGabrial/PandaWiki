import { useCallback, useEffect, useState } from 'react';
import type { MouseEvent } from 'react';
import type { TreeMenuItem } from '@/components/Drag/DragTree/TreeMenu';
import { usesNativeContextMenu } from '../utils/contextMenu';

export function useDocumentContextMenu(resetKey: string) {
  const [state, setState] = useState<{
    position: { top: number; left: number };
    menu: TreeMenuItem[];
  } | null>(null);
  const close = useCallback(() => setState(null), []);
  const open = useCallback((event: MouseEvent, menu: TreeMenuItem[]) => {
    if (usesNativeContextMenu(event.target) || !menu.length) return;
    event.preventDefault();
    event.stopPropagation();
    setState({ position: { top: event.clientY, left: event.clientX }, menu });
  }, []);
  useEffect(close, [resetKey, close]);
  useEffect(() => {
    if (!state) return;
    const inMenu = (target: EventTarget | null) =>
      target instanceof Element &&
      !!target.closest('.cascader-list,.cascader-sub-list');
    const outside = (event: Event) => {
      if (!inMenu(event.target)) close();
    };
    const key = (event: KeyboardEvent) => {
      if (event.key === 'Escape') close();
    };
    window.addEventListener('scroll', outside, true);
    window.addEventListener('resize', close);
    window.addEventListener('blur', close);
    window.addEventListener('popstate', close);
    document.addEventListener('pointerdown', outside, true);
    document.addEventListener('contextmenu', outside);
    document.addEventListener('keydown', key);
    return () => {
      window.removeEventListener('scroll', outside, true);
      window.removeEventListener('resize', close);
      window.removeEventListener('blur', close);
      window.removeEventListener('popstate', close);
      document.removeEventListener('pointerdown', outside, true);
      document.removeEventListener('contextmenu', outside);
      document.removeEventListener('keydown', key);
    };
  }, [state, close]);
  return { state, open, close };
}
