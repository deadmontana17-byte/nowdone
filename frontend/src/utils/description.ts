import type { ChecklistItem, DescriptionBlock, TaskDescription } from '@/types';

/**
 * Task descriptions are stored as JSONB. New tasks use the structured
 * `{ blocks: [...] }` shape; tasks created before this feature use the legacy
 * `{ text: "..." }` shape. Everything in the app reads descriptions through
 * `normalizeDescription` so both keep working.
 */
export function normalizeDescription(raw: unknown): TaskDescription {
  if (raw && typeof raw === 'object') {
    const obj = raw as Record<string, unknown>;

    if (Array.isArray(obj.blocks)) {
      // Trust the stored blocks but defensively drop anything malformed.
      const blocks = (obj.blocks as unknown[]).filter(isDescriptionBlock);
      return { blocks };
    }

    if (typeof obj.text === 'string' && obj.text.trim() !== '') {
      return { blocks: [{ type: 'paragraph', data: { text: obj.text } }] };
    }
  }
  return { blocks: [] };
}

function isDescriptionBlock(b: unknown): b is DescriptionBlock {
  if (!b || typeof b !== 'object') return false;
  const block = b as Record<string, unknown>;
  if (block.type === 'paragraph') {
    return typeof (block.data as { text?: unknown } | undefined)?.text === 'string';
  }
  if (block.type === 'checklist') {
    return Array.isArray((block.data as { items?: unknown } | undefined)?.items);
  }
  return false;
}

/** Concatenated text of every paragraph block (what the edit form shows). */
export function descriptionParagraphText(desc: TaskDescription): string {
  return desc.blocks
    .filter((b): b is Extract<DescriptionBlock, { type: 'paragraph' }> => b.type === 'paragraph')
    .map((b) => b.data.text)
    .join('\n\n');
}

/** All checklist items across every checklist block. */
export function descriptionChecklistItems(desc: TaskDescription): ChecklistItem[] {
  return desc.blocks
    .filter((b): b is Extract<DescriptionBlock, { type: 'checklist' }> => b.type === 'checklist')
    .flatMap((b) => b.data.items);
}

/** Build the stored description from the edit form's two fields. Empty
 * paragraph / empty checklist are omitted so descriptions stay tidy. */
export function buildDescription(paragraph: string, items: ChecklistItem[]): TaskDescription {
  const blocks: DescriptionBlock[] = [];
  if (paragraph.trim() !== '') {
    blocks.push({ type: 'paragraph', data: { text: paragraph } });
  }
  const cleanItems = items
    .map((i) => ({ ...i, text: i.text.trim() }))
    .filter((i) => i.text !== '');
  if (cleanItems.length > 0) {
    blocks.push({ type: 'checklist', data: { items: cleanItems } });
  }
  return { blocks };
}

/** Return a copy of `desc` with one checklist item's `done` flag changed. */
export function toggleChecklistItem(desc: TaskDescription, id: string, done: boolean): TaskDescription {
  return {
    blocks: desc.blocks.map((b) =>
      b.type === 'checklist'
        ? { ...b, data: { items: b.data.items.map((it) => (it.id === id ? { ...it, done } : it)) } }
        : b,
    ),
  };
}

export function newChecklistItem(): ChecklistItem {
  const id =
    typeof crypto !== 'undefined' && 'randomUUID' in crypto
      ? crypto.randomUUID()
      : `id-${Date.now()}-${Math.random().toString(16).slice(2)}`;
  return { id, text: '', done: false };
}

/** True when there is anything worth showing in the detail card. */
export function descriptionIsEmpty(desc: TaskDescription): boolean {
  return desc.blocks.length === 0;
}
