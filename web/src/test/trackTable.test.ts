import { describe, expect, it } from 'vitest';
import {
  applyRecommendedTrackSelection,
  isChineseLanguageCode,
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

  it('matches Chinese language codes across ISO and zh variants', () => {
    expect(isChineseLanguageCode('chi')).toBe(true);
    expect(isChineseLanguageCode('zho')).toBe(true);
    expect(isChineseLanguageCode('zh')).toBe(true);
    expect(isChineseLanguageCode(' zh-cn ')).toBe(true);
    expect(isChineseLanguageCode('ZH-Hans')).toBe(true);
    expect(isChineseLanguageCode('eng')).toBe(false);
  });

  it('resets selection to the first track plus Chinese tracks and keeps a valid default', () => {
    const next = applyRecommendedTrackSelection([
      { id: 'A1', sourceIndex: 0, name: 'English', language: 'eng', selected: false, default: false },
      { id: 'A2', sourceIndex: 1, name: 'Mandarin', language: 'zho', selected: false, default: false },
      { id: 'A3', sourceIndex: 2, name: 'French', language: 'fra', selected: true, default: true },
      { id: 'A4', sourceIndex: 3, name: 'Cantonese', language: 'zh-Hant', selected: false, default: false },
    ]);

    expect(next).toEqual([
      expect.objectContaining({ id: 'A1', selected: true, default: true }),
      expect.objectContaining({ id: 'A2', selected: true, default: false }),
      expect.objectContaining({ id: 'A3', selected: false, default: false }),
      expect.objectContaining({ id: 'A4', selected: true, default: false }),
    ]);
  });

  it('keeps an existing default when it remains inside the recommended selection', () => {
    const next = applyRecommendedTrackSelection([
      { id: 'S1', sourceIndex: 0, name: 'English', language: 'eng', selected: true, default: false },
      { id: 'S2', sourceIndex: 1, name: 'Chinese', language: 'chi', selected: true, default: true },
      { id: 'S3', sourceIndex: 2, name: 'French', language: 'fra', selected: true, default: false },
    ]);

    expect(next).toEqual([
      expect.objectContaining({ id: 'S1', selected: true, default: false }),
      expect.objectContaining({ id: 'S2', selected: true, default: true }),
      expect.objectContaining({ id: 'S3', selected: false, default: false }),
    ]);
  });
});
