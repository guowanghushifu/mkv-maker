import { Button } from './Button';
import { SummaryCard } from './SummaryCard';
import { Fragment } from 'react';
import type { PropsWithChildren } from 'react';
import { getMessages, type Locale } from '../i18n';

export type WorkflowStep = 'login' | 'scan' | 'bdinfo' | 'editor' | 'review';

type LayoutProps = PropsWithChildren<{
  currentStep: WorkflowStep;
  locale: Locale;
  onToggleLocale: () => void;
  context: {
    source: string;
    playlist: string;
    output: string;
    task: string;
  };
}>;

const shellSteps: Exclude<WorkflowStep, 'login'>[] = ['scan', 'bdinfo', 'editor', 'review'];

function getShellMeta(step: Exclude<WorkflowStep, 'login'>, text: ReturnType<typeof getMessages>) {
  return {
    title: text.layout.steps[step],
    subtitle: text.layout.stepDescriptions[step],
  };
}

function renderContextValue(value: string) {
  return value.split(/([./@_-])/g).map((part, index) => (
    <Fragment key={`${part}-${index}`}>
      {part}
      {/[./@_-]/.test(part) ? <wbr /> : null}
    </Fragment>
  ));
}

export function Layout({ currentStep, locale, onToggleLocale, context, children }: LayoutProps) {
  const text = getMessages(locale);
  const activeStep = currentStep === 'login' ? 'scan' : currentStep;
  const meta = getShellMeta(activeStep, text);

  return (
    <div className="admin-shell">
      <aside className="shell-sidebar">
        <div className="shell-brand">
          <div className="shell-brand-mark">MM</div>
          <div className="shell-brand-copy">
            <strong>{text.layout.appTitle}</strong>
            <span>{text.layout.appSubtitle}</span>
          </div>
        </div>
        <nav aria-label={text.layout.shellNavAria} className="shell-nav">
          <ol className="shell-nav-list">
            {shellSteps.map((step) => (
              <li key={step}>
                <span className={`shell-nav-item${step === activeStep ? ' is-active' : ''}`}>
                  <span className="shell-nav-index">{text.layout.stepNumbers[step]}</span>
                  <span>{text.layout.steps[step]}</span>
                </span>
              </li>
            ))}
          </ol>
        </nav>
        <div className="shell-session-card">
          <div className="shell-session-badge">MK</div>
          <div>
            <strong>{text.layout.shellSessionTitle}</strong>
            <p>{text.layout.shellSessionSubtitle}</p>
          </div>
        </div>
      </aside>

      <div className="shell-main">
        <header className="topbar">
          <div className="topbar-copy">
            <h1>{meta.title}</h1>
            <p>{meta.subtitle}</p>
          </div>
          <div className="topbar-actions">
            <Button variant="subtle" className="locale-toggle" onClick={onToggleLocale}>
              {text.layout.localeToggle}
            </Button>
          </div>
        </header>
        <main className="shell-page">
          <section className="workflow-summary-row" aria-label={text.layout.contextTitle}>
            <article className="workflow-summary-card">
              <span className="summary-label">{text.layout.summaryLabels.step}</span>
              <strong className="summary-value">{meta.title}</strong>
            </article>
            <article className="workflow-summary-card">
              <span className="summary-label">{text.layout.summaryLabels.source}</span>
              <strong className="summary-value" title={context.source}>
                {renderContextValue(context.source)}
              </strong>
            </article>
            <article className="workflow-summary-card">
              <span className="summary-label">{text.layout.summaryLabels.playlist}</span>
              <strong className="summary-value" title={context.playlist}>
                {renderContextValue(context.playlist)}
              </strong>
            </article>
            <article className="workflow-summary-card">
              <span className="summary-label">{text.layout.summaryLabels.status}</span>
              <strong className="summary-value" title={context.task}>
                {renderContextValue(context.task)}
              </strong>
            </article>
          </section>
          <section className="workflow-page-grid">
            <div className="workflow-page-main">{children}</div>
            <aside className="workflow-page-aside">
              <SummaryCard
                className="context-card"
                labelClassName="context-card-label"
                valueClassName="context-card-value context-card-value-clamp"
                valueProps={{ title: context.source }}
                label={text.layout.contextLabels.source}
                value={renderContextValue(context.source)}
              />
              <SummaryCard
                className="context-card"
                labelClassName="context-card-label"
                valueClassName="context-card-value context-card-value-clamp"
                valueProps={{ title: context.playlist }}
                label={text.layout.contextLabels.playlist}
                value={renderContextValue(context.playlist)}
              />
              <SummaryCard
                className="context-card"
                labelClassName="context-card-label"
                valueClassName="context-card-value context-card-value-clamp"
                valueProps={{ title: context.output }}
                label={text.layout.contextLabels.output}
                value={renderContextValue(context.output)}
              />
              <SummaryCard
                className="context-card"
                labelClassName="context-card-label"
                valueClassName="context-card-value context-card-value-clamp"
                valueProps={{ title: context.task }}
                label={text.layout.contextLabels.task}
                value={renderContextValue(context.task)}
              />
            </aside>
          </section>
        </main>
      </div>
    </div>
  );
}
