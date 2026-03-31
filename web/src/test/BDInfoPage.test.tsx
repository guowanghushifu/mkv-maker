import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { BDInfoPage } from '../features/bdinfo/BDInfoPage';

const source = {
  id: 'disc-1',
  name: 'The Amateur 2025',
  path: '/bd_input/The Amateur 2025/BDMV',
  type: 'bdmv' as const,
  size: 1,
  modifiedAt: '2026-03-31T12:00:00Z',
};

describe('BDInfoPage', () => {
  it('renders a composed bdinfo workspace with source and sample side panels', () => {
    render(
      <BDInfoPage
        locale="en"
        source={source}
        bdinfoText=""
        parsed={null}
        error={null}
        loading={false}
        onBack={vi.fn()}
        onTextChange={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getByRole('heading', { name: /required bdinfo/i }).closest('.page-panel')).not.toBeNull();
    expect(screen.getByText(/selected source/i).closest('.bdinfo-source-card')).not.toBeNull();
    expect(screen.getByPlaceholderText(/paste full bdinfo text here/i).closest('.bdinfo-composer')).not.toBeNull();
    expect(screen.getByRole('button', { name: /back/i }).closest('.bdinfo-actions')).not.toBeNull();
    expect(screen.getByRole('button', { name: /parse bdinfo and continue/i }).closest('.bdinfo-actions')).not.toBeNull();
    expect(screen.getByText(/bdinfo example/i).closest('.bdinfo-sidebar')).not.toBeNull();
    expect(screen.getByText(/disc title:\s+the amateur 2025/i)).toBeInTheDocument();
    expect(screen.getByText(/presentation graphics\s+english/i)).toBeInTheDocument();
  });

  it('updates the textarea content through onTextChange', () => {
    const onTextChange = vi.fn();
    render(
      <BDInfoPage
        locale="en"
        source={source}
        bdinfoText=""
        parsed={null}
        error={null}
        loading={false}
        onBack={vi.fn()}
        onTextChange={onTextChange}
        onSubmit={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByPlaceholderText(/paste full bdinfo text here/i), {
      target: { value: 'PLAYLIST REPORT' },
    });

    expect(onTextChange).toHaveBeenCalledWith('PLAYLIST REPORT');
  });

  it('renders parsed bdinfo metrics inside a dedicated summary card', () => {
    render(
      <BDInfoPage
        locale="en"
        source={source}
        bdinfoText="PLAYLIST REPORT"
        parsed={{
          playlistName: '00800.MPLS',
          rawText: 'PLAYLIST REPORT',
          audioLabels: ['TrueHD', 'Commentary'],
          subtitleLabels: ['English', 'French'],
        }}
        error={null}
        loading={false}
        onBack={vi.fn()}
        onTextChange={vi.fn()}
        onSubmit={vi.fn()}
      />,
    );

    expect(screen.getAllByText(/00800\.MPLS/i).some((node) => node.closest('.bdinfo-summary-card'))).toBe(true);
    expect(screen.getByText(/audio labels found: 2/i)).toBeInTheDocument();
    expect(screen.getByText(/subtitle labels found: 2/i)).toBeInTheDocument();
  });
});
