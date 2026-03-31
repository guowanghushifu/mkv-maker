import { render } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Layout } from '../components/Layout';

describe('Layout', () => {
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
