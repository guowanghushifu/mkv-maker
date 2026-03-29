import type { PropsWithChildren } from 'react';

export type WorkflowStep = 'login' | 'scan' | 'bdinfo' | 'editor' | 'review' | 'jobs';

type LayoutProps = PropsWithChildren<{
  currentStep: WorkflowStep;
}>;

const stepOrder: WorkflowStep[] = ['login', 'scan', 'bdinfo', 'editor', 'review', 'jobs'];

const stepLabels: Record<WorkflowStep, string> = {
  login: 'Login',
  scan: 'Scan',
  bdinfo: 'BDInfo',
  editor: 'Tracks',
  review: 'Review',
  jobs: 'Jobs',
};

export function Layout({ currentStep, children }: LayoutProps) {
  const activeIndex = stepOrder.indexOf(currentStep);

  return (
    <div className="app-shell">
      <header className="app-header">
        <h1>MKV Remux Tool</h1>
        <p>BDMV workflow with required BDInfo parsing.</p>
      </header>
      <nav aria-label="Workflow steps">
        <ol className="step-list">
          {stepOrder.map((step, index) => {
            const className =
              index === activeIndex ? 'is-active' : index < activeIndex ? 'is-complete' : '';
            return (
              <li key={step} className={className}>
                {stepLabels[step]}
              </li>
            );
          })}
        </ol>
      </nav>
      <main className="page-content">{children}</main>
    </div>
  );
}

