import type { Draft, DraftTrack, ParsedBDInfo, SourceEntry } from '../../api/types';

type ReviewPageProps = {
  source: SourceEntry;
  bdinfo: ParsedBDInfo;
  draft: Draft;
  outputFilename: string;
  outputPath: string;
  submitting: boolean;
  onBack: () => void;
  onSubmit: () => Promise<void> | void;
};

function renderTrackSummary(track: DraftTrack): string {
  const flags: string[] = [];
  if (track.default) {
    flags.push('default');
  }
  if (track.forced) {
    flags.push('forced');
  }
  const suffix = flags.length > 0 ? ` [${flags.join(', ')}]` : '';
  return `${track.name} (${track.language})${suffix}`;
}

export function ReviewPage({
  source,
  bdinfo,
  draft,
  outputFilename,
  outputPath,
  submitting,
  onBack,
  onSubmit,
}: ReviewPageProps) {
  const selectedAudio = draft.audio.filter((track) => track.selected);
  const selectedSubtitles = draft.subtitles.filter((track) => track.selected);

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
          <strong>Filename:</strong> {outputFilename}
        </p>
        <p>
          <strong>Output path:</strong> {outputPath}
        </p>
        <p>
          <strong>Dolby Vision merge enabled:</strong> {draft.dvMergeEnabled ? 'Yes' : 'No'}
        </p>
      </div>

      <h3>Final Track List and Order</h3>
      <ol className="ordered-track-list">
        <li>Video: {draft.video.name}</li>
        {selectedAudio.map((track) => (
          <li key={`audio-${track.id}`}>Audio: {renderTrackSummary(track)}</li>
        ))}
        {selectedSubtitles.map((track) => (
          <li key={`sub-${track.id}`}>Subtitle: {renderTrackSummary(track)}</li>
        ))}
      </ol>

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

