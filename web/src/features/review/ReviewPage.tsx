import type { Draft, DraftTrack, Job, ParsedBDInfo, SourceEntry } from '../../api/types';
import { StatusBadge } from '../../components/StatusBadge';

type ReviewPageProps = {
  source: SourceEntry;
  bdinfo: ParsedBDInfo;
  draft: Draft;
  outputFilename: string;
  outputPath: string;
  submitting: boolean;
  startDisabled: boolean;
  submitError: string | null;
  currentJob: Job | null;
  currentLog: string;
  onBack: () => void;
  onStartNextRemux: () => Promise<void> | void;
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
  startDisabled,
  submitError,
  currentJob,
  currentLog,
  onBack,
  onStartNextRemux,
  onSubmit,
}: ReviewPageProps) {
  const selectedAudio = draft.audio.filter((track) => track.selected);
  const selectedSubtitles = draft.subtitles.filter((track) => track.selected);
  const progressPercent =
    currentJob?.status === 'succeeded'
      ? 100
      : Math.max(0, Math.min(100, currentJob?.progressPercent ?? 0));

  return (
    <section className="panel">
      <h2>Review</h2>
      <p>Confirm metadata and start the remux.</p>
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

      <div className="review-actions">
        <div className="review-actions-primary">
          <button type="button" onClick={onBack}>
            Back
          </button>
          <button type="button" onClick={() => void onSubmit()} disabled={submitting || startDisabled}>
            {submitting ? 'Starting Remux...' : 'Start Remux'}
          </button>
        </div>
        {currentJob ? (
          <div className="review-actions-secondary">
            <button
              type="button"
              onClick={() => void onStartNextRemux()}
              disabled={currentJob.status === 'running'}
            >
              Start Next Remux
            </button>
          </div>
        ) : null}
      </div>
      {submitError ? <p className="error-text">{submitError}</p> : null}

      {currentJob ? (
        <section className="info-box current-job-panel">
          <div className="row">
            <h3>Current Remux</h3>
            <StatusBadge status={currentJob.status} />
          </div>
          <p>
            <strong>Output:</strong> {currentJob.outputName}
          </p>
          <p>
            <strong>Path:</strong> {currentJob.outputPath}
          </p>
          <div className="current-job-progress">
            <div className="row">
              <h4>Progress</h4>
              <strong>{progressPercent}%</strong>
            </div>
            <div
              className="progress-bar"
              role="progressbar"
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={progressPercent}
            >
              <div className="progress-bar-fill" style={{ width: `${progressPercent}%` }} />
            </div>
          </div>
          {currentJob.commandPreview ? (
            <div className="current-job-command">
              <h4>Command Preview</h4>
              <pre className="command-preview scroll-panel">{currentJob.commandPreview}</pre>
            </div>
          ) : null}
          {currentJob.message ? <p className="error-text">{currentJob.message}</p> : null}
          <pre className="job-log scroll-panel">{currentLog || 'Waiting for log output...'}</pre>
        </section>
      ) : null}
    </section>
  );
}
