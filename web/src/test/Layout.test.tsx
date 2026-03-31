import { render } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Layout } from '../components/Layout';

describe('Layout', () => {
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
    expect(container.querySelector('.workflow-page-aside')).not.toBeNull();
    expect(container.querySelectorAll('.workflow-page-aside .context-card')).toHaveLength(4);
    expect(container.querySelectorAll('.workflow-page-aside .context-card-value-clamp')).toHaveLength(4);
    expect(container.querySelectorAll('.shell-nav-index')).toHaveLength(4);
    expect(container.querySelector('.shell-sidebar')).not.toBeNull();
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
    expect(container.querySelectorAll('.context-card-value-clamp wbr').length).toBeGreaterThan(0);
  });

  it('inserts wrap opportunities for long session context values', () => {
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
        <div />
      </Layout>
    );

    const clampedValues = container.querySelectorAll('.context-card-value-clamp');
    expect(clampedValues.length).toBeGreaterThan(0);
    expect(container.querySelectorAll('.context-card-value-clamp wbr').length).toBeGreaterThan(0);
  });
});
