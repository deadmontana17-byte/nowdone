export interface User {
  id: string;
  first_name?: string;
  has_pin: boolean;
  current_streak: number;
  max_streak: number;
  timezone: string; // IANA name, e.g. "Europe/Moscow"; "UTC" by default
}

export interface Attachment {
  type: 'image' | 'video' | 'file';
  url: string;
  name: string;
}

export interface RecurrenceRule {
  // "weekdays" = every Mon–Fri; "yearly" = same date each year.
  frequency: 'daily' | 'weekdays' | 'weekly' | 'monthly' | 'yearly';
  interval?: number;
  week_days?: number[];
}

/** One checklist entry inside a task description. */
export interface ChecklistItem {
  id: string;
  text: string;
  done: boolean;
}

/** A block of structured task description content (stored in `description.blocks`). */
export type DescriptionBlock =
  | { type: 'paragraph'; data: { text: string } }
  | { type: 'checklist'; data: { items: ChecklistItem[] } };

/** Structured task description: rich paragraphs + interactive checklists.
 * Legacy tasks store `{ text: string }` instead — see normalizeDescription(). */
export interface TaskDescription {
  blocks: DescriptionBlock[];
}

/** What actually sits in the `description` JSONB column: the new structured
 * shape, the legacy `{ text }` shape, or `{}`. Always read via normalizeDescription(). */
export type StoredDescription = TaskDescription | { text?: string };

export interface Task {
  id: string;
  user_id: string;
  type_id?: string | null;
  title: string;
  description: StoredDescription;
  attachments: Attachment[];
  date: string; // YYYY-MM-DD
  is_done: boolean;
  reminder_time?: string | null;
  is_recurring: boolean;
  recurrence_rule?: RecurrenceRule | null;
  created_at: string;
}

export interface TaskType {
  id: string;
  user_id: string;
  emoji: string;
  name: string;
  created_at: string;
}

export interface Note {
  id: string;
  user_id: string;
  title: string;
  content: Record<string, unknown>;
  attachments: Attachment[];
  is_hidden: boolean;
  created_at: string;
}
