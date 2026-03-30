import type { ParsedBDInfo, SourceEntry } from '../../api/types';
import { getMessages, type Locale } from '../../i18n';

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
    <section className="panel">
      <h2>{text.bdinfo.title}</h2>
      <p>
        {text.bdinfo.selectedSource}: <strong>{source.name}</strong>
      </p>
      <p>{text.bdinfo.description}</p>
      <textarea
        value={bdinfoText}
        onChange={(event) => onTextChange(event.target.value)}
        rows={12}
        placeholder={text.bdinfo.placeholder}
      />
      {error ? <p className="error-text">{error}</p> : null}
      {parsed ? (
        <div className="info-box">
          <p>
            {text.bdinfo.playlist}: <strong>{parsed.playlistName}</strong>
          </p>
          <p>{text.bdinfo.audioLabelsFound}: {parsed.audioLabels.length}</p>
          <p>{text.bdinfo.subtitleLabelsFound}: {parsed.subtitleLabels.length}</p>
        </div>
      ) : null}
      <div className="row">
        <button type="button" onClick={onBack}>
          {text.bdinfo.backButton}
        </button>
        <button type="button" onClick={() => void onSubmit()} disabled={!bdinfoText.trim() || loading}>
          {loading ? text.bdinfo.submittingButton : text.bdinfo.submitButton}
        </button>
      </div>
    </section>
  );
}
