import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Switch } from '../components/Switch';

describe('Switch', () => {
  it('renders a switch with the current checked state', () => {
    render(<Switch aria-label="Include English Atmos" checked onChange={vi.fn()} />);

    expect(screen.getByRole('switch', { name: /include english atmos/i })).toHaveAttribute('aria-checked', 'true');
  });

  it('invokes onChange when activated and ignores clicks while disabled', () => {
    const onChange = vi.fn();
    const { rerender } = render(<Switch aria-label="Default Commentary" checked={false} onChange={onChange} />);

    fireEvent.click(screen.getByRole('switch', { name: /default commentary/i }));
    expect(onChange).toHaveBeenCalledTimes(1);

    rerender(<Switch aria-label="Default Commentary" checked={false} disabled onChange={onChange} />);

    fireEvent.click(screen.getByRole('switch', { name: /default commentary/i }));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('switch', { name: /default commentary/i })).toBeDisabled();
  });
});
