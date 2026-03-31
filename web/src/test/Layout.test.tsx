import { render } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Layout } from '../components/Layout';

describe('Layout', () => {
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
