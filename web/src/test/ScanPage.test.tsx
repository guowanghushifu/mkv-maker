import { fireEvent, render, screen } from '@testing-library/react';
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

const secondSource = {
  ...longPathSource,
  id: 'disc-2',
  name: 'Another Disc',
  path: '/bd_input/Another Disc/BDMV',
};

const isoSource = {
  ...longPathSource,
  id: 'iso-1',
  name: 'Nightcrawler',
  path: '/bd_input/Nightcrawler.iso',
  type: 'iso' as const,
};

describe('ScanPage', () => {
  it('renders scan content inside the workspace card and toolbar layout', () => {
    const { container } = render(
      <ScanPage
        locale="en"
        loading={false}
        error={null}
        sources={[longPathSource, secondSource]}
        selectedSourceId={null}
        onReleaseMountedISOs={vi.fn()}
        onScan={vi.fn()}
        onSelectSource={vi.fn()}
        onNext={vi.fn()}
      />,
    );

    expect(container.querySelector('.workspace-card.scan-workspace')).not.toBeNull();
    expect(container.querySelector('.workspace-toolbar')).not.toBeNull();
    expect(container.querySelector('.source-grid')).not.toBeNull();
  });

  it('renders sources as selectable cards before the table details', () => {
    const { container } = render(
      <ScanPage
        locale="en"
        loading={false}
        error={null}
        sources={[longPathSource]}
        selectedSourceId={null}
        onReleaseMountedISOs={vi.fn()}
        onScan={vi.fn()}
        onSelectSource={vi.fn()}
        onNext={vi.fn()}
      />,
    );

    expect(container.querySelector('.source-grid')).not.toBeNull();
    expect(container.querySelectorAll('.source-card')).toHaveLength(1);
    expect(screen.getByText(longPathSource.name).closest('.source-card')).not.toBeNull();
    expect(screen.getByText(/BDMV Directory/i)).toBeInTheDocument();
    expect(screen.getByText(/1\.0 KB/i)).toBeInTheDocument();
  });

  it('uses a dedicated two-up grid layout for source cards', () => {
    const { container } = render(
      <ScanPage
        locale="en"
        loading={false}
        error={null}
        sources={[longPathSource, secondSource]}
        selectedSourceId={null}
        onReleaseMountedISOs={vi.fn()}
        onScan={vi.fn()}
        onSelectSource={vi.fn()}
        onNext={vi.fn()}
      />,
    );

    expect(container.querySelector('.source-grid.source-grid-two-up')).not.toBeNull();
  });

  it('renders long source names with hoverable full text metadata in the source card', () => {
    render(
      <ScanPage
        locale="en"
        loading={false}
        error={null}
        sources={[longPathSource]}
        selectedSourceId={null}
        onReleaseMountedISOs={vi.fn()}
        onScan={vi.fn()}
        onSelectSource={vi.fn()}
        onNext={vi.fn()}
      />,
    );

    const nameText = screen.getByText(longPathSource.name);
    expect(nameText).toHaveAttribute('title', longPathSource.name);
    expect(nameText).toHaveClass('source-card-title');
  });

  it('renders long source paths with hoverable full text metadata in the source card', () => {
    render(
      <ScanPage
        locale="en"
        loading={false}
        error={null}
        sources={[longPathSource]}
        selectedSourceId={null}
        onReleaseMountedISOs={vi.fn()}
        onScan={vi.fn()}
        onSelectSource={vi.fn()}
        onNext={vi.fn()}
      />,
    );

    const pathText = screen.getByText(longPathSource.path);
    expect(pathText).toHaveAttribute('title', longPathSource.path);
    expect(pathText).toHaveClass('source-card-path');
  });

  it('marks the selected source card with an active state', () => {
    const { container } = render(
      <ScanPage
        locale="en"
        loading={false}
        error={null}
        sources={[longPathSource]}
        selectedSourceId={longPathSource.id}
        onReleaseMountedISOs={vi.fn()}
        onScan={vi.fn()}
        onSelectSource={vi.fn()}
        onNext={vi.fn()}
      />,
    );

    expect(container.querySelector('.source-card.is-selected')).not.toBeNull();
  });

  it('keeps next disabled until a source is selected and forwards selection changes', () => {
    const onSelectSource = vi.fn();
    const onNext = vi.fn();
    const { rerender } = render(
      <ScanPage
        locale="en"
        loading={false}
        error={null}
        sources={[longPathSource, secondSource]}
        selectedSourceId={null}
        onReleaseMountedISOs={vi.fn()}
        onScan={vi.fn()}
        onSelectSource={onSelectSource}
        onNext={onNext}
      />,
    );

    const nextButton = screen.getByRole('button', { name: /continue to bdinfo/i });
    expect(nextButton).toBeDisabled();

    fireEvent.click(screen.getByLabelText(new RegExp(secondSource.name, 'i')));
    expect(onSelectSource).toHaveBeenCalledWith(secondSource.id);
    expect(onNext).not.toHaveBeenCalled();

    rerender(
      <ScanPage
        locale="en"
        loading={false}
        error={null}
        sources={[longPathSource, secondSource]}
        selectedSourceId={secondSource.id}
        onReleaseMountedISOs={vi.fn()}
        onScan={vi.fn()}
        onSelectSource={onSelectSource}
        onNext={onNext}
      />,
    );

    const enabledNextButton = screen.getByRole('button', { name: /continue to bdinfo/i });
    expect(enabledNextButton).not.toBeDisabled();
    fireEvent.click(enabledNextButton);
    expect(onNext).toHaveBeenCalledTimes(1);
  });

  it('renders the release button before scan sources and shows the ISO badge', () => {
    const ScanPageAny = ScanPage as any;

    render(
      <ScanPageAny
        locale="en"
        loading={false}
        releasingMountedISOs={false}
        error={null}
        sources={[isoSource]}
        selectedSourceId={null}
        onReleaseMountedISOs={vi.fn()}
        onScan={vi.fn()}
        onSelectSource={vi.fn()}
        onNext={vi.fn()}
      />,
    );

    expect(Array.from(document.querySelectorAll('.workspace-toolbar button')).map((node) => node.textContent?.trim())).toEqual([
      'Release Mounted ISOs',
      'Scan Sources',
      'Continue to BDInfo',
    ]);
    expect(screen.getByText(/ISO File/i)).toBeInTheDocument();
  });
});
