import type { SourceEntry } from '../../api/types';

type ScanPageProps = {
  loading: boolean;
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
  loading,
  sources,
  selectedSourceId,
  onScan,
  onSelectSource,
  onNext,
}: ScanPageProps) {
  const typeLabel = (type: SourceEntry['type']) => {
    if (type === 'bdmv') {
      return 'BDMV Folder';
    }
    return type;
  };

  return (
    <section className="panel">
      <h2>Scan Sources</h2>
      <p>Only extracted BDMV folders are accepted as workflow input.</p>
      <div className="row">
        <button type="button" onClick={() => void onScan()} disabled={loading}>
          {loading ? 'Scanning...' : 'Scan Sources (POST /api/sources/scan)'}
        </button>
        <button type="button" onClick={onNext} disabled={!selectedSourceId}>
          Continue to BDInfo
        </button>
      </div>
      {sources.length === 0 ? (
        <p className="muted-text">No sources yet. Run scan to discover BDMV directories.</p>
      ) : (
        <div className="source-table-wrap">
          <table className="source-table">
            <thead>
              <tr>
                <th>Select</th>
                <th>Name</th>
                <th>Type</th>
                <th>Path</th>
                <th>Size</th>
                <th>Modified</th>
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
                      aria-label={`Select ${source.name}`}
                    />
                  </td>
                  <td>{source.name}</td>
                  <td>{typeLabel(source.type)}</td>
                  <td>{source.path}</td>
                  <td>{formatBytes(source.size)}</td>
                  <td>{new Date(source.modifiedAt).toLocaleString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
