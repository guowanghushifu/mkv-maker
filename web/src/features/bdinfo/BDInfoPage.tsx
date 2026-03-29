import type { ParsedBDInfo, SourceEntry } from '../../api/types';

type BDInfoPageProps = {
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
  source,
  bdinfoText,
  parsed,
  error,
  loading,
  onBack,
  onTextChange,
  onSubmit,
}: BDInfoPageProps) {
  return (
    <section className="panel">
      <h2>Required BDInfo</h2>
      <p>
        Selected source: <strong>{source.name}</strong>
      </p>
      <p>Paste the BDInfo log. This step is required and cannot be skipped.</p>
      <textarea
        value={bdinfoText}
        onChange={(event) => onTextChange(event.target.value)}
        rows={12}
        placeholder="Paste full BDInfo text here"
      />
      {error ? <p className="error-text">{error}</p> : null}
      {parsed ? (
        <div className="info-box">
          <p>
            Playlist: <strong>{parsed.playlistName}</strong>
          </p>
          <p>Audio labels found: {parsed.audioLabels.length}</p>
          <p>Subtitle labels found: {parsed.subtitleLabels.length}</p>
        </div>
      ) : null}
      <div className="row">
        <button type="button" onClick={onBack}>
          Back
        </button>
        <button type="button" onClick={() => void onSubmit()} disabled={!bdinfoText.trim() || loading}>
          {loading ? 'Parsing...' : 'Parse BDInfo and Continue'}
        </button>
      </div>
    </section>
  );
}

