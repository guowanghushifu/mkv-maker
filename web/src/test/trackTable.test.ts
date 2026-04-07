import { describe, expect, it } from 'vitest';
import {
  moveTrackRow,
  setExclusiveDefault,
  toggleTrackSelected,
} from '../features/draft/trackTable';

describe('trackTable helpers', () => {
  it('inserts the dragged row before the target when moving upward', () => {
    const next = moveTrackRow(
      [
        { id: 'A1', sourceIndex: 0, name: 'English', language: 'eng', selected: true, default: true },
        { id: 'A2', sourceIndex: 1, name: 'Commentary', language: 'eng', selected: true, default: false },
      ],
      'A2',
      'A1',
    );
    expect(next.map((track) => track.id)).toEqual(['A2', 'A1']);
  });

  it('inserts the dragged row before the target when moving downward', () => {
    const next = moveTrackRow(
      [
        { id: 'A1', sourceIndex: 0, name: 'English', language: 'eng', selected: true, default: true },
        { id: 'A2', sourceIndex: 1, name: 'Commentary', language: 'eng', selected: true, default: false },
        { id: 'A3', sourceIndex: 2, name: 'French', language: 'fra', selected: true, default: false },
      ],
      'A1',
      'A3',
    );
    expect(next.map((track) => track.id)).toEqual(['A2', 'A1', 'A3']);
  });

  it('keeps only one default track in the group', () => {
    const next = setExclusiveDefault(
      [
        { id: 'A1', sourceIndex: 0, name: 'English', language: 'eng', selected: true, default: true },
        { id: 'A2', sourceIndex: 1, name: 'Commentary', language: 'eng', selected: true, default: false },
      ],
      'A2',
    );
    expect(next.find((track) => track.id === 'A1')?.default).toBe(false);
    expect(next.find((track) => track.id === 'A2')?.default).toBe(true);
  });

  it('clears default when a track is deselected', () => {
    const next = toggleTrackSelected(
      [{ id: 'A1', sourceIndex: 0, name: 'English', language: 'eng', selected: true, default: true }],
      'A1',
    );
    expect(next[0].selected).toBe(false);
    expect(next[0].default).toBe(false);
  });
});
