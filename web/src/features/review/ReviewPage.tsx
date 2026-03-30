import type { Draft, DraftTrack, Job, ParsedBDInfo, SourceEntry } from '../../api/types';
import { getMessages, type Locale } from '../../i18n';
import { StatusBadge } from '../../components/StatusBadge';

type ReviewPageProps = {
  locale?: Locale;
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

function renderTrackSummaryWithLocale(track: DraftTrack, locale: Locale): string {
  const text = getMessages(locale);
  const flags: string[] = [];
  if (track.default) {
    flags.push(text.review.defaultFlag);
  }
  if (track.forced) {
    flags.push(text.review.forcedFlag);
  }
  const suffix = flags.length > 0 ? ` [${flags.join(', ')}]` : '';
  return `${track.name} (${track.language})${suffix}`;
}

export function ReviewPage({
  locale = 'zh',
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
  const text = getMessages(locale);
  const selectedAudio = draft.audio.filter((track) => track.selected);
  const selectedSubtitles = draft.subtitles.filter((track) => track.selected);
  const progressPercent =
    currentJob?.status === 'succeeded'
      ? 100
      : Math.max(0, Math.min(100, currentJob?.progressPercent ?? 0));

  return (
    <section className="panel">
      <h2>{text.review.title}</h2>
      <p>{text.review.description}</p>
      <div className="info-box">
        <p>
          <strong>{text.review.source}:</strong> {source.name}
        </p>
        <p>
          <strong>{text.review.playlist}:</strong> {bdinfo.playlistName}
        </p>
        <p>
          <strong>{text.review.filename}:</strong> {outputFilename}
        </p>
        <p>
          <strong>{text.review.outputPath}:</strong> {outputPath}
        </p>
        <p>
          <strong>{text.review.dolbyVisionMergeEnabled}:</strong> {draft.dvMergeEnabled ? text.review.yes : text.review.no}
        </p>
      </div>

      <h3>{text.review.finalTrackList}</h3>
      <ol className="ordered-track-list">
        <li>{text.review.video}: {draft.video.name}</li>
        {selectedAudio.map((track) => (
          <li key={`audio-${track.id}`}>{text.review.audio}: {renderTrackSummaryWithLocale(track, locale)}</li>
        ))}
        {selectedSubtitles.map((track) => (
          <li key={`sub-${track.id}`}>{text.review.subtitle}: {renderTrackSummaryWithLocale(track, locale)}</li>
        ))}
      </ol>

      <div className="review-actions">
        <div className="review-actions-primary">
          <button type="button" onClick={onBack}>
            {text.review.backButton}
          </button>
          <button type="button" onClick={() => void onSubmit()} disabled={submitting || startDisabled}>
            {submitting ? text.review.startingRemuxButton : text.review.startRemuxButton}
          </button>
        </div>
        {currentJob ? (
          <div className="review-actions-secondary">
            <button
              type="button"
              onClick={() => void onStartNextRemux()}
              disabled={currentJob.status === 'running'}
            >
              {text.review.startNextRemuxButton}
            </button>
          </div>
        ) : null}
      </div>
      {submitError ? <p className="error-text">{submitError}</p> : null}

      {currentJob ? (
        <section className="info-box current-job-panel">
          <div className="row">
            <h3>{text.review.currentRemux}</h3>
            <StatusBadge status={currentJob.status} locale={locale} />
          </div>
          <p>
            <strong>{text.review.output}:</strong> {currentJob.outputName}
          </p>
          <p>
            <strong>{text.review.path}:</strong> {currentJob.outputPath}
          </p>
          <div className="current-job-progress">
            <div className="row">
              <h4>{text.review.progress}</h4>
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
              <h4>{text.review.commandPreview}</h4>
              <pre className="command-preview scroll-panel">{currentJob.commandPreview}</pre>
            </div>
          ) : null}
          {currentJob.message ? <p className="error-text">{currentJob.message}</p> : null}
          <pre className="job-log scroll-panel">{currentLog || text.review.waitingForLogOutput}</pre>
        </section>
      ) : null}
    </section>
  );
}
