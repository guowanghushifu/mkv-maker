import type { Draft, DraftTrack } from '../../api/types';

type TrackEditorPageProps = {
  draft: Draft;
  filenamePreview?: string;
  outputFilename?: string;
  onFilenameChange?: (value: string) => void;
  onChange: (next: Draft) => void;
  onBack?: () => void;
  onNext?: () => void;
};

function findPreviousSelectedIndex(tracks: DraftTrack[], index: number): number {
  for (let i = index - 1; i >= 0; i -= 1) {
    if (tracks[i].selected) {
      return i;
    }
  }
  return -1;
}

function findNextSelectedIndex(tracks: DraftTrack[], index: number): number {
  for (let i = index + 1; i < tracks.length; i += 1) {
    if (tracks[i].selected) {
      return i;
    }
  }
  return -1;
}

function swapTracks(tracks: DraftTrack[], from: number, to: number): DraftTrack[] {
  const next = [...tracks];
  [next[from], next[to]] = [next[to], next[from]];
  return next;
}

export function TrackEditorPage({
  draft,
  filenamePreview,
  outputFilename,
  onFilenameChange,
  onChange,
  onBack,
  onNext,
}: TrackEditorPageProps) {
  const updateVideoName = (name: string) => {
    onChange({ ...draft, video: { ...draft.video, name } });
  };

  const updateTitle = (title: string) => {
    onChange({ ...draft, title });
  };

  const updateAudioTrack = (trackId: string, updater: (track: DraftTrack) => DraftTrack) => {
    const nextAudio = draft.audio.map((track) => (track.id === trackId ? updater(track) : track));
    onChange({ ...draft, audio: nextAudio });
  };

  const updateSubtitleTrack = (trackId: string, updater: (track: DraftTrack) => DraftTrack) => {
    const nextSubtitles = draft.subtitles.map((track) =>
      track.id === trackId ? updater(track) : track
    );
    onChange({ ...draft, subtitles: nextSubtitles });
  };

  const toggleAudioSelected = (trackId: string) => {
    updateAudioTrack(trackId, (track) => {
      const selected = !track.selected;
      return {
        ...track,
        selected,
        default: selected ? track.default : false,
      };
    });
  };

  const toggleSubtitleSelected = (trackId: string) => {
    updateSubtitleTrack(trackId, (track) => {
      const selected = !track.selected;
      return {
        ...track,
        selected,
        default: selected ? track.default : false,
      };
    });
  };

  const setDefaultAudio = (trackId: string) => {
    onChange({
      ...draft,
      audio: draft.audio.map((track) => ({
        ...track,
        default: track.id === trackId && track.selected,
      })),
    });
  };

  const setDefaultSubtitle = (trackId: string) => {
    onChange({
      ...draft,
      subtitles: draft.subtitles.map((track) => ({
        ...track,
        default: track.id === trackId && track.selected,
      })),
    });
  };

  const moveAudioUp = (index: number) => {
    if (!draft.audio[index].selected) {
      return;
    }
    const previousIndex = findPreviousSelectedIndex(draft.audio, index);
    if (previousIndex < 0) {
      return;
    }
    onChange({ ...draft, audio: swapTracks(draft.audio, index, previousIndex) });
  };

  const moveAudioDown = (index: number) => {
    if (!draft.audio[index].selected) {
      return;
    }
    const nextIndex = findNextSelectedIndex(draft.audio, index);
    if (nextIndex < 0) {
      return;
    }
    onChange({ ...draft, audio: swapTracks(draft.audio, index, nextIndex) });
  };

  const moveSubtitleUp = (index: number) => {
    if (!draft.subtitles[index].selected) {
      return;
    }
    const previousIndex = findPreviousSelectedIndex(draft.subtitles, index);
    if (previousIndex < 0) {
      return;
    }
    onChange({ ...draft, subtitles: swapTracks(draft.subtitles, index, previousIndex) });
  };

  const moveSubtitleDown = (index: number) => {
    if (!draft.subtitles[index].selected) {
      return;
    }
    const nextIndex = findNextSelectedIndex(draft.subtitles, index);
    if (nextIndex < 0) {
      return;
    }
    onChange({ ...draft, subtitles: swapTracks(draft.subtitles, index, nextIndex) });
  };

  return (
    <section className="panel">
      <h2>Track Editor</h2>

      <div className="stack">
        <label htmlFor="draft-title">Title</label>
        <input
          id="draft-title"
          type="text"
          value={draft.title || ''}
          onChange={(event) => updateTitle(event.target.value)}
        />
        <label htmlFor="video-track-name">Video track name</label>
        <input
          id="video-track-name"
          type="text"
          value={draft.video.name}
          onChange={(event) => updateVideoName(event.target.value)}
        />
      </div>

      <p>
        Video source attributes: {draft.video.codec} / {draft.video.resolution}
        {draft.video.hdrType ? ` / ${draft.video.hdrType}` : ''}
      </p>

      {typeof filenamePreview === 'string' ? (
        <div className="info-box">
          <p>
            <strong>Live filename preview:</strong> {filenamePreview}
          </p>
          {onFilenameChange ? (
            <>
              <label htmlFor="output-filename">Output filename</label>
              <input
                id="output-filename"
                type="text"
                value={outputFilename || ''}
                onChange={(event) => onFilenameChange(event.target.value)}
              />
            </>
          ) : null}
        </div>
      ) : null}

      <h3>Audio</h3>
      <ul className="track-list">
        {draft.audio.map((track, index) => (
          <li key={track.id}>
            <label htmlFor={`audio-name-${track.id}`}>Name</label>
            <input
              id={`audio-name-${track.id}`}
              type="text"
              value={track.name}
              onChange={(event) =>
                updateAudioTrack(track.id, (current) => ({
                  ...current,
                  name: event.target.value,
                }))
              }
            />

            <label htmlFor={`audio-lang-${track.id}`}>Language</label>
            <input
              id={`audio-lang-${track.id}`}
              type="text"
              value={track.language}
              onChange={(event) =>
                updateAudioTrack(track.id, (current) => ({ ...current, language: event.target.value }))
              }
            />

            <label>
              <input
                type="checkbox"
                checked={track.selected}
                onChange={() => toggleAudioSelected(track.id)}
              />
              Selected
            </label>
            <label>
              <input
                type="radio"
                name="default-audio"
                checked={track.default}
                onChange={() => setDefaultAudio(track.id)}
                disabled={!track.selected}
              />
              Default
            </label>
            <div className="row">
              <button
                type="button"
                onClick={() => moveAudioUp(index)}
                disabled={!track.selected || findPreviousSelectedIndex(draft.audio, index) < 0}
              >
                Move {track.name} up
              </button>
              <button
                type="button"
                onClick={() => moveAudioDown(index)}
                disabled={!track.selected || findNextSelectedIndex(draft.audio, index) < 0}
              >
                Move {track.name} down
              </button>
            </div>
          </li>
        ))}
      </ul>

      <h3>Subtitles</h3>
      {draft.subtitles.length === 0 ? (
        <p className="muted-text">No subtitles found in this draft.</p>
      ) : (
        <ul className="track-list">
          {draft.subtitles.map((track, index) => (
            <li key={track.id}>
              <label htmlFor={`subtitle-name-${track.id}`}>Name</label>
              <input
                id={`subtitle-name-${track.id}`}
                type="text"
                value={track.name}
                onChange={(event) =>
                  updateSubtitleTrack(track.id, (current) => ({ ...current, name: event.target.value }))
                }
              />

              <label htmlFor={`subtitle-lang-${track.id}`}>Language</label>
              <input
                id={`subtitle-lang-${track.id}`}
                type="text"
                value={track.language}
                onChange={(event) =>
                  updateSubtitleTrack(track.id, (current) => ({ ...current, language: event.target.value }))
                }
              />

              <label>
                <input
                  type="checkbox"
                  checked={track.selected}
                  onChange={() => toggleSubtitleSelected(track.id)}
                />
                Selected
              </label>
              <label>
                <input
                  type="radio"
                  name="default-subtitle"
                  checked={track.default}
                  onChange={() => setDefaultSubtitle(track.id)}
                  disabled={!track.selected}
                />
                Default
              </label>
              <div className="row">
                <button
                  type="button"
                  onClick={() => moveSubtitleUp(index)}
                  disabled={!track.selected || findPreviousSelectedIndex(draft.subtitles, index) < 0}
                >
                  Move {track.name} up
                </button>
                <button
                  type="button"
                  onClick={() => moveSubtitleDown(index)}
                  disabled={!track.selected || findNextSelectedIndex(draft.subtitles, index) < 0}
                >
                  Move {track.name} down
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}

      {onBack || onNext ? (
        <div className="row">
          {onBack ? (
            <button type="button" onClick={onBack}>
              Back
            </button>
          ) : null}
          {onNext ? (
            <button type="button" onClick={onNext}>
              Continue to Review
            </button>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}
