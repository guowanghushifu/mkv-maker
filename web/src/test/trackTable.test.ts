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
        { id: 'a1', name: 'English', language: 'eng', selected: true, default: true },
        { id: 'a2', name: 'Commentary', language: 'eng', selected: true, default: false },
      ],
      'a2',
      'a1',
    );
    expect(next.map((track) => track.id)).toEqual(['a2', 'a1']);
  });

  it('inserts the dragged row before the target when moving downward', () => {
    const next = moveTrackRow(
      [
        { id: 'a1', name: 'English', language: 'eng', selected: true, default: true },
        { id: 'a2', name: 'Commentary', language: 'eng', selected: true, default: false },
        { id: 'a3', name: 'French', language: 'fra', selected: true, default: false },
      ],
      'a1',
      'a3',
    );
    expect(next.map((track) => track.id)).toEqual(['a2', 'a1', 'a3']);
  });

  it('keeps only one default track in the group', () => {
    const next = setExclusiveDefault(
      [
        { id: 'a1', name: 'English', language: 'eng', selected: true, default: true },
        { id: 'a2', name: 'Commentary', language: 'eng', selected: true, default: false },
      ],
      'a2',
    );
    expect(next.find((track) => track.id === 'a1')?.default).toBe(false);
    expect(next.find((track) => track.id === 'a2')?.default).toBe(true);
  });

  it('clears default when a track is deselected', () => {
    const next = toggleTrackSelected(
      [{ id: 'a1', name: 'English', language: 'eng', selected: true, default: true }],
      'a1',
    );
    expect(next[0].selected).toBe(false);
    expect(next[0].default).toBe(false);
  });
});
