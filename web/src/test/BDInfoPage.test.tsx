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
  it('renders bdinfo as a split workspace with support cards in the aside column', () => {
    const { container } = render(
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

    expect(container.querySelector('.workspace-card.bdinfo-workspace')).not.toBeNull();
    expect(container.querySelector('.bdinfo-layout')).not.toBeNull();
    expect(container.querySelector('.bdinfo-sidebar.supporting-card')).not.toBeNull();
    expect(screen.getByText(/bdinfo example/i).closest('.supporting-card')).not.toBeNull();
  });

  it('renders the bdinfo sample below the actions instead of inside the sidebar', () => {
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
    expect(screen.getByText(/bdinfo example/i).closest('.bdinfo-sidebar')).toBeNull();
    expect(screen.getByText(/bdinfo example/i).closest('.bdinfo-sample')).not.toBeNull();
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

  it('preserves bdinfo submit gating and action callbacks in the redesigned workspace', () => {
    const onBack = vi.fn();
    const onSubmit = vi.fn();
    const { rerender } = render(
      <BDInfoPage
        locale="en"
        source={source}
        bdinfoText=""
        parsed={null}
        error={null}
        loading={false}
        onBack={onBack}
        onTextChange={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    const submitButton = screen.getByRole('button', { name: /parse bdinfo and continue/i });
    expect(submitButton).toBeDisabled();
    fireEvent.click(screen.getByRole('button', { name: /back/i }));
    expect(onBack).toHaveBeenCalledTimes(1);

    rerender(
      <BDInfoPage
        locale="en"
        source={source}
        bdinfoText="PLAYLIST REPORT"
        parsed={null}
        error={null}
        loading={true}
        onBack={onBack}
        onTextChange={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    expect(screen.getByRole('button', { name: /parsing/i })).toBeDisabled();

    rerender(
      <BDInfoPage
        locale="en"
        source={source}
        bdinfoText="PLAYLIST REPORT"
        parsed={null}
        error={null}
        loading={false}
        onBack={onBack}
        onTextChange={vi.fn()}
        onSubmit={onSubmit}
      />,
    );

    const enabledSubmitButton = screen.getByRole('button', { name: /parse bdinfo and continue/i });
    expect(enabledSubmitButton).not.toBeDisabled();
    fireEvent.click(enabledSubmitButton);
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });
});
