import type { Draft, ParsedBDInfo, SourceEntry } from '../../api/types';

type ReviewPageProps = {
  source: SourceEntry;
  bdinfo: ParsedBDInfo;
  draft: Draft;
  submitting: boolean;
  onBack: () => void;
  onSubmit: () => Promise<void> | void;
};

export function ReviewPage({ source, bdinfo, draft, submitting, onBack, onSubmit }: ReviewPageProps) {
  const selectedAudio = draft.audio.filter((track) => track.selected);
  const selectedSubtitles = draft.subtitles.filter((track) => track.selected);
  const outputBaseName = draft.title || bdinfo.discTitle || source.name;

  return (
    <section className="panel">
      <h2>Review</h2>
      <p>Confirm metadata and queue the remux job.</p>
      <div className="info-box">
        <p>
          <strong>Source:</strong> {source.name}
        </p>
        <p>
          <strong>Playlist:</strong> {bdinfo.playlistName}
        </p>
        <p>
          <strong>Output:</strong> {outputBaseName.replace(/\s+/g, '.')}
          .mkv
        </p>
        <p>
          <strong>Selected audio tracks:</strong> {selectedAudio.length}
        </p>
        <p>
          <strong>Selected subtitles:</strong> {selectedSubtitles.length}
        </p>
      </div>
      <div className="row">
        <button type="button" onClick={onBack}>
          Back
        </button>
        <button type="button" onClick={() => void onSubmit()} disabled={submitting}>
          {submitting ? 'Queueing...' : 'Queue Remux Job'}
        </button>
      </div>
    </section>
  );
}

