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
