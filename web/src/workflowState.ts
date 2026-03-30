import type { Draft, ParsedBDInfo, SourceEntry } from './api/types';
import type { WorkflowStep } from './components/Layout';

export type PersistedWorkflowState = {
  step: WorkflowStep;
  sources: SourceEntry[];
  selectedSourceId: string | null;
  bdinfoText: string;
  parsedBDInfo: ParsedBDInfo | null;
  draft: Draft | null;
  filenamePreview: string;
  outputFilename: string;
  filenameEdited: boolean;
};

export const workflowStorageKey = 'mkv-maker-workflow';

export function loadStoredWorkflowState(): PersistedWorkflowState | null {
  if (typeof window === 'undefined') {
    return null;
  }
  const raw = window.localStorage.getItem(workflowStorageKey);
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw) as PersistedWorkflowState;
  } catch {
    return null;
  }
}

export function saveStoredWorkflowState(value: PersistedWorkflowState | null): void {
  if (typeof window === 'undefined') {
    return;
  }
  if (!value) {
    window.localStorage.removeItem(workflowStorageKey);
    return;
  }
  window.localStorage.setItem(workflowStorageKey, JSON.stringify(value));
}
