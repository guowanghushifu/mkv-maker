import type { ParsedBDInfo, SourceEntry } from '../../api/types';
import { Button } from '../../components/Button';
import { SummaryCard } from '../../components/SummaryCard';
import { getMessages, type Locale } from '../../i18n';
import { sampleBDInfo } from './sampleBDInfo';

type BDInfoPageProps = {
  locale?: Locale;
  source: SourceEntry;
  bdinfoText: string;
  parsed: ParsedBDInfo | null;
  error: string | null;
  loading: boolean;
  onBack: () => void;
  onTextChange: (value: string) => void;
  onSubmit: () => Promise<void> | void;
};

export function BDInfoPage({
  locale = 'zh',
  source,
  bdinfoText,
  parsed,
  error,
  loading,
  onBack,
  onTextChange,
  onSubmit,
}: BDInfoPageProps) {
  const text = getMessages(locale);

  return (
    <section className="panel page-panel bdinfo-panel">
      <div className="panel-header">
        <div>
          <h2>{text.bdinfo.title}</h2>
          <p className="panel-description">{text.bdinfo.description}</p>
        </div>
      </div>

      <div className="bdinfo-layout">
        <div className="bdinfo-composer">
          <textarea
            value={bdinfoText}
            onChange={(event) => onTextChange(event.target.value)}
            rows={16}
            placeholder={text.bdinfo.placeholder}
          />
          {error ? <p className="error-text">{error}</p> : null}
          <div className="row bdinfo-actions">
            <Button variant="subtle" onClick={onBack}>
              {text.bdinfo.backButton}
            </Button>
            <Button onClick={() => void onSubmit()} disabled={!bdinfoText.trim() || loading}>
              {loading ? text.bdinfo.submittingButton : text.bdinfo.submitButton}
            </Button>
          </div>
          <section className="bdinfo-sample">
            <h3>{text.bdinfo.sampleTitle}</h3>
            <pre>{sampleBDInfo}</pre>
          </section>
        </div>

        <aside className="bdinfo-sidebar">
          <SummaryCard
            className="bdinfo-source-card"
            label={text.bdinfo.selectedSource}
            value={source.name}
          >
            <span className="source-card-path" title={source.path}>
              {source.path}
            </span>
          </SummaryCard>

          {parsed ? (
            <SummaryCard
              className="bdinfo-summary-card"
              label={text.bdinfo.playlist}
              value={parsed.playlistName}
            >
              <p>{text.bdinfo.audioLabelsFound}: {parsed.audioLabels.length}</p>
              <p>{text.bdinfo.subtitleLabelsFound}: {parsed.subtitleLabels.length}</p>
            </SummaryCard>
          ) : null}
        </aside>
      </div>
    </section>
  );
}
