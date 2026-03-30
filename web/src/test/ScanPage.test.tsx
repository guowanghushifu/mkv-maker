import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { ScanPage } from '../features/sources/ScanPage';

const longPathSource = {
  id: 'disc-1',
  name: '倩女幽魂.A.Chinese.Ghost.Story.AKA.Sien.lui.yau.wan.1987.FRA.UHD.Blu-ray.2160p.DV.HDR.HEVC.DTS-HD.MA.7.1-sh@CHDBits',
  path: '/bd_input/倩女幽魂.A.Chinese.Ghost.Story.AKA.Sien.lui.yau.wan.1987.FRA.UHD.Blu-ray.2160p.DV.HDR.HEVC.DTS-HD.MA.7.1-sh@CHDBits',
  type: 'bdmv' as const,
  size: 1024,
  modifiedAt: '2026-03-29T12:00:00Z',
};

describe('ScanPage', () => {
  it('keeps fixed columns compact so name and path can use remaining space', () => {
    const { container } = render(
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

    const columns = Array.from(container.querySelectorAll('.source-table col'));
    expect(columns).toHaveLength(6);
    expect(columns[1]).not.toHaveClass('col-name');
    expect(columns[3]).not.toHaveClass('col-path');
    expect(columns[0]).toHaveClass('col-select');
    expect(columns[2]).toHaveClass('col-type');
    expect(columns[4]).toHaveClass('col-size');
    expect(columns[5]).toHaveClass('col-modified');
  });

  it('renders long source names with hoverable full text metadata', () => {
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

    const nameText = screen.getByText(longPathSource.name);
    expect(nameText).toHaveAttribute('title', longPathSource.name);
    expect(nameText).toHaveClass('source-name-text');
  });

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
