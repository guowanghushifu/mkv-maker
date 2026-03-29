import type { SourceEntry } from '../../api/types';

type ScanPageProps = {
  loading: boolean;
  sources: SourceEntry[];
  selectedSourceId: string | null;
  onScan: () => Promise<void> | void;
  onSelectSource: (sourceId: string) => void;
  onNext: () => void;
};

export function ScanPage({
  loading,
  sources,
  selectedSourceId,
  onScan,
  onSelectSource,
  onNext,
}: ScanPageProps) {
  return (
    <section className="panel">
      <h2>Scan Sources</h2>
      <p>Only extracted BDMV folders are accepted as workflow input.</p>
      <div className="row">
        <button type="button" onClick={() => void onScan()} disabled={loading}>
          {loading ? 'Scanning...' : 'Scan for Sources'}
        </button>
        <button type="button" onClick={onNext} disabled={!selectedSourceId}>
          Continue to BDInfo
        </button>
      </div>
      {sources.length === 0 ? (
        <p className="muted-text">No sources yet. Run scan to discover BDMV directories.</p>
      ) : (
        <ul className="source-list">
          {sources.map((source) => (
            <li key={source.id}>
              <label>
                <input
                  type="radio"
                  name="source"
                  checked={source.id === selectedSourceId}
                  onChange={() => onSelectSource(source.id)}
                />
                <span>{source.name}</span>
                <small>{source.path}</small>
              </label>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

