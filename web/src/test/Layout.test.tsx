import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Layout } from '../components/Layout';

describe('Layout', () => {
  it('renders the wide and compact summary-card class hooks used by the tightened radius system', () => {
    const { container } = render(
      <Layout
        currentStep="scan"
        locale="en"
        onToggleLocale={vi.fn()}
        context={{
          source: 'Nightcrawler Disc',
          playlist: 'Waiting',
          output: 'Nightcrawler - 2160p.mkv',
          task: 'Ready',
        }}
      >
        <div>scan body</div>
      </Layout>
    );

    expect(container.querySelectorAll('.workflow-summary-card.summary-card-wide')).toHaveLength(2);
    expect(container.querySelectorAll('.workflow-summary-card.summary-card-compact')).toHaveLength(2);
    expect(container.querySelectorAll('.workflow-summary-card .summary-value.summary-value-clamp')).toHaveLength(2);
  });

  it('renders summary cards and the aside context stack required by the light shell', () => {
    const { container } = render(
      <Layout
        currentStep="scan"
        locale="en"
        onToggleLocale={vi.fn()}
        context={{
          source: 'Nightcrawler Disc',
          playlist: 'Waiting',
          output: 'Nightcrawler - 2160p.mkv',
          task: 'Ready',
        }}
      >
        <div>scan body</div>
      </Layout>
    );

    expect(container.querySelector('.admin-shell')).not.toBeNull();
    expect(container.querySelector('.shell-brand')).not.toBeNull();
    expect(container.querySelector('.shell-brand-mark')).not.toBeNull();
    expect(container.querySelector('.shell-brand-copy')).not.toBeNull();
    expect(container.querySelector('.shell-session-card')).not.toBeNull();
    expect(container.querySelector('.workflow-summary-row')).not.toBeNull();
    expect(container.querySelector('.shell-page')).not.toBeNull();
    expect(container.querySelector('.workflow-page-main')).not.toBeNull();
    expect(container.querySelectorAll('.workflow-summary-card')).toHaveLength(4);
    expect(container.querySelectorAll('.workflow-summary-card .summary-label')).toHaveLength(4);
    expect(container.querySelectorAll('.workflow-summary-card .summary-value')).toHaveLength(4);
    expect(container.querySelectorAll('.workflow-summary-card.summary-card-wide')).toHaveLength(2);
    expect(container.querySelectorAll('.workflow-summary-card.summary-card-compact')).toHaveLength(2);
    expect(container.querySelectorAll('.workflow-summary-card .summary-value.summary-value-clamp')).toHaveLength(2);
    expect(container.querySelector('.workflow-page-aside')).toBeNull();
    expect(container.querySelectorAll('.context-card')).toHaveLength(0);
    expect(container.querySelectorAll('.shell-nav-index')).toHaveLength(4);
    expect(container.querySelector('.shell-sidebar')).not.toBeNull();
    expect(container.querySelector('.topbar-actions')).not.toBeNull();
    expect(container.querySelector('.workflow-page-grid')).not.toBeNull();
    expect(container.querySelector('.workflow-summary-row')).toHaveTextContent('Selected Source');
    expect(container.querySelector('.workflow-summary-row')).toHaveTextContent('Playlist');
    expect(container.querySelector('.workflow-summary-row')).toHaveTextContent('Output');
    expect(container.querySelector('.workflow-summary-row')).toHaveTextContent('Task State');
    expect(container.querySelector('.workflow-summary-row')).not.toHaveTextContent('Current Step');
  });

  it('renders the post-login shell with sidebar items and wrap opportunities in the context panel', () => {
    const { container } = render(
      <Layout
        currentStep="review"
        locale="zh"
        onToggleLocale={vi.fn()}
        context={{
          source: '夜行者.Nightcrawler.2014.V2.2160p.USA.Blu-ray.DV.HDR.HEVC.TrueHD.7.1.Atmos-LINMENG@CHDBits',
          playlist: '00800.MPLS',
          output: '夜行者.Nightcrawler.2014.V2.2160p.USA.Blu-ray.DV.HDR.HEVC.TrueHD.7.1.Atmos-LINMENG@CHDBits.mkv',
          task: '就绪',
        }}
      >
        <div>review content</div>
      </Layout>
    );

    expect(container.querySelector('.admin-shell')).not.toBeNull();
    expect(container.querySelectorAll('.shell-nav-item')).toHaveLength(4);
    expect(container.querySelector('.shell-nav-item.is-active')).not.toBeNull();
    expect(container.querySelector('.topbar')).not.toBeNull();
    expect(container.querySelectorAll('.workflow-summary-row .summary-value wbr').length).toBeGreaterThan(0);
    expect(container.querySelectorAll('.workflow-summary-row .summary-value.summary-value-clamp wbr').length).toBeGreaterThan(0);
    expect(container.querySelector('.workflow-page-aside')).toBeNull();
  });

  it('keeps shell markers and summary cards structurally present after the shell radius refactor', () => {
    const { container } = render(
      <Layout
        currentStep="review"
        locale="zh"
        onToggleLocale={vi.fn()}
        context={{
          source: '夜行者.Nightcrawler.2014.V2.2160p.USA.Blu-ray.DV.HDR.HEVC.TrueHD.7.1.Atmos-LINMENG@CHDBits',
          playlist: '00800.MPLS',
          output: '夜行者.Nightcrawler.2014.V2.2160p.USA.Blu-ray.DV.HDR.HEVC.TrueHD.7.1.Atmos-LINMENG@CHDBits.mkv',
          task: '就绪',
        }}
      >
        <div>review content</div>
      </Layout>
    );

    expect(container.querySelectorAll('.shell-nav-index')).toHaveLength(4);
    expect(container.querySelector('.shell-session-badge')).not.toBeNull();
    expect(container.querySelectorAll('.workflow-summary-card')).toHaveLength(4);
  });

  it('shows Back to Scan before the locale toggle only on bdinfo, editor, and review steps', () => {
    const onBackToScan = vi.fn();
    const context = {
      source: 'Nightcrawler Disc',
      playlist: '00800.MPLS',
      output: 'Nightcrawler - 2160p.mkv',
      task: 'Ready',
    };
    const { rerender } = render(
      <Layout currentStep="scan" locale="en" onToggleLocale={vi.fn()} onBackToScan={onBackToScan} context={context}>
        <div>scan body</div>
      </Layout>
    );

    expect(screen.queryByRole('button', { name: /back to scan/i })).toBeNull();

    rerender(
      <Layout currentStep="bdinfo" locale="en" onToggleLocale={vi.fn()} onBackToScan={onBackToScan} context={context}>
        <div>bdinfo body</div>
      </Layout>
    );

    expect(screen.getByRole('button', { name: /back to scan/i })).toBeInTheDocument();
    expect(Array.from(document.querySelectorAll('.topbar-actions button')).map((node) => node.textContent?.trim())).toEqual([
      'Back to Scan',
      '中文 / EN',
    ]);

    rerender(
      <Layout currentStep="editor" locale="en" onToggleLocale={vi.fn()} onBackToScan={onBackToScan} context={context}>
        <div>editor body</div>
      </Layout>
    );
    expect(screen.getByRole('button', { name: /back to scan/i })).toBeInTheDocument();

    rerender(
      <Layout currentStep="review" locale="en" onToggleLocale={vi.fn()} onBackToScan={onBackToScan} context={context}>
        <div>review body</div>
      </Layout>
    );
    expect(screen.getByRole('button', { name: /back to scan/i })).toBeInTheDocument();
  });
});
