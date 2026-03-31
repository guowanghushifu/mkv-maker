import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { LoginPage } from '../features/auth/LoginPage';

describe('LoginPage', () => {
  it('renders a centered login layout shell for the password form', () => {
    render(<LoginPage locale="en" onSuccess={vi.fn()} />);

    expect(screen.getByRole('heading', { name: /login/i }).closest('section')).toHaveClass('login-panel');
    expect(screen.getByRole('heading', { name: /login/i }).closest('.login-panel-body')).not.toBeNull();
    expect(screen.getByLabelText(/password/i).closest('form')).toHaveClass('login-form');
    expect(screen.getByLabelText(/password/i).closest('.login-form-field')).not.toBeNull();
  });

  it('requires a password before submitting', () => {
    const onSuccess = vi.fn();
    render(<LoginPage locale="en" onSuccess={onSuccess} />);

    fireEvent.click(screen.getByRole('button', { name: /continue/i }));

    expect(onSuccess).not.toHaveBeenCalled();
    expect(screen.getByText(/password is required/i)).toBeInTheDocument();
  });
});
