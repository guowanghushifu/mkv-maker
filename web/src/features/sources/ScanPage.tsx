import type { SourceEntry } from '../../api/types';
import { Button } from '../../components/Button';
import { getMessages, type Locale } from '../../i18n';

type ScanPageProps = {
  locale?: Locale;
  loading: boolean;
  error?: string | null;
  sources: SourceEntry[];
  selectedSourceId: string | null;
  onScan: () => Promise<void> | void;
  onSelectSource: (sourceId: string) => void;
  onNext: () => void;
};

function formatBytes(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} KB`;
  }
  if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

function formatModifiedDate(value: string, locale: Locale): string {
  return new Date(value).toLocaleString(locale === 'zh' ? 'zh-CN' : 'en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

export function ScanPage({
  locale = 'zh',
  loading,
  error,
  sources,
  selectedSourceId,
  onScan,
  onSelectSource,
  onNext,
}: ScanPageProps) {
  const text = getMessages(locale);

  const typeLabel = (type: SourceEntry['type']) => {
    if (type === 'bdmv') {
      return text.scan.typeBDMV;
    }
    return type;
  };

  return (
    <section className="panel page-panel scan-panel">
      <div className="panel-header">
        <div>
          <h2>{text.scan.title}</h2>
          <p className="panel-description">{text.scan.subtitle}</p>
        </div>
        <div className="panel-toolbar">
          <Button onClick={() => void onScan()} disabled={loading}>
            {loading ? text.scan.scanningButton : text.scan.scanButton}
          </Button>
          <Button onClick={onNext} disabled={!selectedSourceId}>
            {text.scan.nextButton}
          </Button>
        </div>
      </div>
      {error ? <p className="error-text">{error}</p> : null}
      {sources.length === 0 ? (
        <div className="empty-state">
          <p className="muted-text">{text.scan.empty}</p>
        </div>
      ) : (
        <div className="source-grid source-grid-two-up">
          {sources.map((source) => {
            const isSelected = source.id === selectedSourceId;
            return (
              <label
                key={source.id}
                className={`source-card${isSelected ? ' is-selected' : ''}`}
              >
                <input
                  type="radio"
                  name="source"
                  checked={isSelected}
                  onChange={() => onSelectSource(source.id)}
                  aria-label={text.scan.selectSource(source.name)}
                />
                <div className="source-card-head">
                  <span className="source-card-badge">{typeLabel(source.type)}</span>
                  <span className="source-card-size">{formatBytes(source.size)}</span>
                </div>
                <div className="source-card-body">
                  <strong className="source-card-title" title={source.name}>
                    {source.name}
                  </strong>
                  <span className="source-card-path" title={source.path}>
                    {source.path}
                  </span>
                </div>
                <div className="source-card-footer">
                  <span className="source-card-meta-label">{text.scan.columns.modified}</span>
                  <span className="source-card-meta-value">{formatModifiedDate(source.modifiedAt, locale)}</span>
                </div>
              </label>
            );
          })}
        </div>
      )}
    </section>
  );
}
