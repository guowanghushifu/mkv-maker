import type { Draft } from '../../api/types';

type TrackEditorPageProps = {
  draft: Draft;
  onChange: (next: Draft) => void;
  onBack?: () => void;
  onNext?: () => void;
};

export function TrackEditorPage({ draft, onChange, onBack, onNext }: TrackEditorPageProps) {
  const moveAudioUp = (index: number) => {
    if (index === 0) {
      return;
    }

    const nextAudio = [...draft.audio];
    [nextAudio[index - 1], nextAudio[index]] = [nextAudio[index], nextAudio[index - 1]];
    onChange({ ...draft, audio: nextAudio });
  };

  const toggleAudioSelected = (trackId: string) => {
    const nextAudio = draft.audio.map((track) => {
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
    onChange({ ...draft, audio: nextAudio });
  };

  const setDefaultAudio = (trackId: string) => {
    const nextAudio = draft.audio.map((track) => ({
      ...track,
      default: track.id === trackId && track.selected,
    }));
    onChange({ ...draft, audio: nextAudio });
  };

  const toggleSubtitleSelected = (trackId: string) => {
    const nextSubtitles = draft.subtitles.map((track) =>
      track.id === trackId ? { ...track, selected: !track.selected } : track
    );
    onChange({ ...draft, subtitles: nextSubtitles });
  };

  return (
    <section className="panel">
      <h2>Track Editor</h2>
      <p>
        Video: {draft.video.name} ({draft.video.codec} / {draft.video.resolution}
        {draft.video.hdrType ? ` / ${draft.video.hdrType}` : ''})
      </p>

      <h3>Audio</h3>
      <ul className="track-list">
        {draft.audio.map((track, index) => (
          <li key={track.id}>
            <strong>{track.name}</strong>
            <small>{track.language}</small>
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
            <button type="button" onClick={() => moveAudioUp(index)} disabled={index === 0}>
              Move {track.name} up
            </button>
          </li>
        ))}
      </ul>

      <h3>Subtitles</h3>
      {draft.subtitles.length === 0 ? (
        <p className="muted-text">No subtitles found in this draft.</p>
      ) : (
        <ul className="track-list">
          {draft.subtitles.map((track) => (
            <li key={track.id}>
              <strong>{track.name}</strong>
              <small>{track.language}</small>
              <label>
                <input
                  type="checkbox"
                  checked={track.selected}
                  onChange={() => toggleSubtitleSelected(track.id)}
                />
                Selected
              </label>
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

