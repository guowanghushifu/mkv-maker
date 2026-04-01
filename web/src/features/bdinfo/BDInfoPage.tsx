import type { ParsedBDInfo, SourceEntry } from '../../api/types';
import { Button } from '../../components/Button';
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
  bdinfoText,
  error,
  loading,
  onBack,
  onTextChange,
  onSubmit,
}: BDInfoPageProps) {
  const text = getMessages(locale);

  return (
    <section className="workspace-card page-panel bdinfo-workspace">
      <div className="workspace-header">
        <div>
          <h2>{text.bdinfo.title}</h2>
          <p className="panel-description">{text.bdinfo.description}</p>
        </div>
      </div>

      <div className="bdinfo-layout bdinfo-layout-single">
        <div className="bdinfo-composer supporting-card">
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
        </div>
      </div>

      <section className="bdinfo-sample supporting-card">
        <h3>{text.bdinfo.sampleTitle}</h3>
        <pre className="bdinfo-sample-pre">{sampleBDInfo}</pre>
      </section>
    </section>
  );
}
