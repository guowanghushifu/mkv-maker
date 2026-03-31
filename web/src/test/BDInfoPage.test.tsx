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
  it('renders a separated actions area and a bdinfo sample container', () => {
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

    expect(screen.getByRole('button', { name: /back/i }).closest('.bdinfo-actions')).not.toBeNull();
    expect(screen.getByRole('button', { name: /parse bdinfo and continue/i }).closest('.bdinfo-actions')).not.toBeNull();
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
});
