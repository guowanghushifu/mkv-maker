import { useEffect, useRef } from 'react';
import type { Draft, DraftTrack, Job, ParsedBDInfo, SourceEntry } from '../../api/types';
import { Button } from '../../components/Button';
import { getMessages, type Locale } from '../../i18n';
import { SummaryCard } from '../../components/SummaryCard';
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
  const logRef = useRef<HTMLPreElement | null>(null);
  const selectedAudio = draft.audio.filter((track) => track.selected);
  const selectedSubtitles = draft.subtitles.filter((track) => track.selected);
  const progressPercent =
    currentJob?.status === 'succeeded'
      ? 100
      : Math.max(0, Math.min(100, currentJob?.progressPercent ?? 0));

  useEffect(() => {
    if (!logRef.current) {
      return;
    }
    logRef.current.scrollTop = logRef.current.scrollHeight;
  }, [currentLog, currentJob?.id]);

  return (
    <section className="workspace-card page-panel review-panel review-workspace">
      <div className="workspace-header">
        <div>
          <h2>{text.review.title}</h2>
          <p className="panel-description">{text.review.description}</p>
        </div>
      </div>

      <section className="review-track-panel review-section-card">
        <div className="section-heading">
          <h3>{text.review.finalTrackList}</h3>
          <p className="muted-text">{outputPath}</p>
        </div>
        <ol className="ordered-track-list">
          <li>{text.review.video}: {draft.video.name}</li>
          {selectedAudio.map((track) => (
            <li key={`audio-${track.id}`}>{text.review.audio}: {renderTrackSummaryWithLocale(track, locale)}</li>
          ))}
          {selectedSubtitles.map((track) => (
            <li key={`sub-${track.id}`}>{text.review.subtitle}: {renderTrackSummaryWithLocale(track, locale)}</li>
          ))}
        </ol>
      </section>

      <div className="review-actions">
        <div className="review-actions-primary">
          <Button variant="subtle" onClick={onBack}>
            {text.review.backButton}
          </Button>
          <Button onClick={() => void onSubmit()} disabled={submitting || startDisabled}>
            {submitting ? text.review.startingRemuxButton : text.review.startRemuxButton}
          </Button>
        </div>
        <div className="review-actions-secondary">
          <Button
            variant="subtle"
            onClick={() => void onStartNextRemux()}
            disabled={currentJob?.status === 'running'}
          >
            {text.review.startNextRemuxButton}
          </Button>
        </div>
      </div>
      {submitError ? <p className="error-text">{submitError}</p> : null}

      {currentJob ? (
        <section className="job-console current-job-panel review-section-card">
          <div className="job-console-header">
            <div>
              <p className="context-kicker">{text.review.currentRemux}</p>
              <h3>{currentJob.outputName}</h3>
            </div>
            <StatusBadge status={currentJob.status} locale={locale} />
          </div>
          <div className="job-console-grid">
            <SummaryCard className="review-summary-card" label={text.review.output} value={currentJob.outputName} />
            <SummaryCard className="review-summary-card" label={text.review.path} value={currentJob.outputPath} />
          </div>
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
            <div className="current-job-command console-block">
              <h4>{text.review.commandPreview}</h4>
              <pre className="command-preview scroll-panel">{currentJob.commandPreview}</pre>
            </div>
          ) : null}
          {currentJob.message ? <p className="error-text">{currentJob.message}</p> : null}
          <div className="console-block">
            <h4>{text.review.logOutput}</h4>
            <pre ref={logRef} className="job-log scroll-panel">{currentLog || text.review.waitingForLogOutput}</pre>
          </div>
        </section>
      ) : null}
    </section>
  );
}
