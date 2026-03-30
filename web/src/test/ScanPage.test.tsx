import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ScanPage } from '../features/sources/ScanPage';

const longPathSource = {
  id: 'disc-1',
  name: 'A Chinese Ghost Story',
  path: '/bd_input/倩女幽魂.A.Chinese.Ghost.Story.AKA.Sien.lui.yau.wan.1987.FRA.UHD.Blu-ray.2160p.DV.HDR.HEVC.DTS-HD.MA.7.1-sh@CHDBits',
  type: 'bdmv' as const,
  size: 1024,
  modifiedAt: '2026-03-29T12:00:00Z',
};

describe('ScanPage', () => {
  it('renders long source paths with hoverable full text metadata', () => {
    render(
      <ScanPage
        loading={false}
        error={null}
        sources={[longPathSource]}
        selectedSourceId={null}
        onScan={vi.fn()}
        onSelectSource={vi.fn()}
        onNext={vi.fn()}
      />,
    );

    const pathText = screen.getByText(longPathSource.path);
    expect(pathText).toHaveAttribute('title', longPathSource.path);
    expect(pathText).toHaveClass('source-path-text');
  });
});
