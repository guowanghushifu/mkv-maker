import type { SourceEntry } from '../../api/types';
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
    <section className="panel">
      <h2>{text.scan.title}</h2>
      <p>{text.scan.subtitle}</p>
      <div className="row">
        <button type="button" onClick={() => void onScan()} disabled={loading}>
          {loading ? text.scan.scanningButton : text.scan.scanButton}
        </button>
        <button type="button" onClick={onNext} disabled={!selectedSourceId}>
          {text.scan.nextButton}
        </button>
      </div>
      {error ? <p className="error-text">{error}</p> : null}
      {sources.length === 0 ? (
        <p className="muted-text">{text.scan.empty}</p>
      ) : (
        <div className="source-table-wrap">
          <table className="source-table">
            <colgroup>
              <col className="col-select" />
              <col />
              <col className="col-type" />
              <col />
              <col className="col-size" />
              <col className="col-modified" />
            </colgroup>
            <thead>
              <tr>
                <th>{text.scan.columns.select}</th>
                <th>{text.scan.columns.name}</th>
                <th>{text.scan.columns.type}</th>
                <th>{text.scan.columns.path}</th>
                <th>{text.scan.columns.size}</th>
                <th>{text.scan.columns.modified}</th>
              </tr>
            </thead>
            <tbody>
              {sources.map((source) => (
                <tr key={source.id}>
                  <td>
                    <input
                      type="radio"
                      name="source"
                      checked={source.id === selectedSourceId}
                      onChange={() => onSelectSource(source.id)}
                      aria-label={text.scan.selectSource(source.name)}
                    />
                  </td>
                  <td className="source-name-cell">
                    <span className="source-name-text" title={source.name}>
                      {source.name}
                    </span>
                  </td>
                  <td>{typeLabel(source.type)}</td>
                  <td className="source-path-cell">
                    <span className="source-path-text" title={source.path}>
                      {source.path}
                    </span>
                  </td>
                  <td className="source-table-nowrap">{formatBytes(source.size)}</td>
                  <td className="source-table-nowrap">
                    {new Date(source.modifiedAt).toLocaleString(locale === 'zh' ? 'zh-CN' : 'en-US')}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
