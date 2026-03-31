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

const stepOrder: WorkflowStep[] = ['login', 'scan', 'bdinfo', 'editor', 'review'];

function renderContextValue(value: string) {
  return value.split(/([./@_-])/g).map((part, index) => (
    <Fragment key={`${part}-${index}`}>
      {part}
      {/[./@_-]/.test(part) ? <wbr /> : null}
    </Fragment>
  ));
}

export function Layout({ currentStep, locale, onToggleLocale, context, children }: LayoutProps) {
  const activeIndex = stepOrder.indexOf(currentStep);
  const text = getMessages(locale);

  return (
    <div className="app-shell">
      <header className="app-header app-hero">
        <div className="app-header-top">
          <div className="app-hero-copy">
            <h1>{text.layout.appTitle}</h1>
            <p>{text.layout.appSubtitle}</p>
          </div>
          <div className="app-hero-actions">
            <Button variant="subtle" className="locale-toggle" onClick={onToggleLocale}>
              {text.layout.localeToggle}
            </Button>
          </div>
        </div>
      </header>
      <section className="workflow-context">
        <div className="workflow-context-header">
          <div>
            <p className="context-kicker">{text.layout.contextTitle}</p>
          </div>
        </div>
        <nav aria-label={text.layout.workflowStepsAria}>
          <ol className="step-list">
            {stepOrder.map((step, index) => {
              const className =
                index === activeIndex ? 'is-active' : index < activeIndex ? 'is-complete' : '';
              return (
                <li key={step} className={className}>
                  <span className="step-index">{String(index + 1).padStart(2, '0')}</span>
                  <span className="step-copy">
                    <span className="step-label">{text.layout.steps[step]}</span>
                  </span>
                </li>
              );
            })}
          </ol>
        </nav>
        <div className="context-card-grid">
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
        </div>
      </section>
      <main className="page-content">{children}</main>
    </div>
  );
}
