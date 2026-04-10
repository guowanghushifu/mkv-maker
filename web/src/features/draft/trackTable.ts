import type { DraftTrack } from '../../api/types';

export function moveTrackRow(tracks: DraftTrack[], sourceId: string, targetId: string): DraftTrack[] {
  const fromIndex = tracks.findIndex((track) => track.id === sourceId);
  const toIndex = tracks.findIndex((track) => track.id === targetId);
  if (fromIndex < 0 || toIndex < 0 || fromIndex === toIndex) {
    return tracks;
  }
  const next = [...tracks];
  const [moved] = next.splice(fromIndex, 1);
  const insertIndex = fromIndex < toIndex ? toIndex - 1 : toIndex;
  next.splice(insertIndex, 0, moved);
  return next;
}

export function setExclusiveDefault(tracks: DraftTrack[], trackId: string): DraftTrack[] {
  return tracks.map((track) => ({
    ...track,
    default: track.id === trackId && track.selected,
  }));
}

export function toggleTrackSelected(tracks: DraftTrack[], trackId: string): DraftTrack[] {
  return tracks.map((track) => {
    if (track.id !== trackId) {
      return track;
    }
    const selected = !track.selected;
    return {
      ...track,
      selected,
      default: selected ? track.default : false,
    };
  });
}

export function isChineseLanguageCode(value: string): boolean {
  const normalized = value.trim().toLowerCase();
  return normalized === 'chi' || normalized === 'zho' || normalized === 'zh' || normalized.startsWith('zh-');
}

export function applyRecommendedTrackSelection(tracks: DraftTrack[]): DraftTrack[] {
  if (tracks.length === 0) {
    return tracks;
  }

  const selectedIndexes = new Set<number>([0]);
  tracks.forEach((track, index) => {
    if (isChineseLanguageCode(track.language)) {
      selectedIndexes.add(index);
    }
  });

  const selectedDefaultIndex = tracks.findIndex((track, index) => selectedIndexes.has(index) && track.default);
  const firstSelectedIndex = tracks.findIndex((_track, index) => selectedIndexes.has(index));
  const defaultIndex = selectedDefaultIndex >= 0 ? selectedDefaultIndex : firstSelectedIndex;

  const recommended = tracks.map((track, index) => {
    const selected = selectedIndexes.has(index);
    return {
      ...track,
      selected,
      default: selected && index === defaultIndex,
    };
  });

  return [
    ...recommended.filter((track) => track.selected),
    ...recommended.filter((track) => !track.selected),
  ];
}
