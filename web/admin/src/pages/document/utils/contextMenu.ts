// Keep text editing, links and controls on the native browser context menu.
export function usesNativeContextMenu(target: EventTarget | null): boolean {
  return (
    target instanceof Element &&
    !!target.closest(
      'input,textarea,button,a,[contenteditable="true"],[contenteditable=""],[data-native-context-menu]',
    )
  );
}

export function contextSelection(
  id: string,
  selected: string[],
  selectable: boolean,
): string[] {
  if (!selectable) return [];
  return selected.includes(id) ? selected : [id];
}
