import type { PropsWithChildren } from 'react';
import { getMessages, type Locale } from '../i18n';

export type WorkflowStep = 'login' | 'scan' | 'bdinfo' | 'editor' | 'review';

type LayoutProps = PropsWithChildren<{
  currentStep: WorkflowStep;
  locale: Locale;
  onToggleLocale: () => void;
}>;

const stepOrder: WorkflowStep[] = ['login', 'scan', 'bdinfo', 'editor', 'review'];

export function Layout({ currentStep, locale, onToggleLocale, children }: LayoutProps) {
  const activeIndex = stepOrder.indexOf(currentStep);
  const text = getMessages(locale);

  return (
    <div className="app-shell">
      <header className="app-header">
        <div className="app-header-top">
          <div>
            <h1>{text.layout.appTitle}</h1>
            <p>{text.layout.appSubtitle}</p>
          </div>
          <button type="button" className="locale-toggle" onClick={onToggleLocale}>
            {text.layout.localeToggle}
          </button>
        </div>
      </header>
      <nav aria-label={text.layout.workflowStepsAria}>
        <ol className="step-list">
          {stepOrder.map((step, index) => {
            const className =
              index === activeIndex ? 'is-active' : index < activeIndex ? 'is-complete' : '';
            return (
              <li key={step} className={className}>
                {text.layout.steps[step]}
              </li>
            );
          })}
        </ol>
      </nav>
      <main className="page-content">{children}</main>
    </div>
  );
}
