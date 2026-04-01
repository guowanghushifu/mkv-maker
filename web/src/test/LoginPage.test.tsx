import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { LoginPage } from '../features/auth/LoginPage';

describe('LoginPage', () => {
  it('renders the standalone light login card without the admin shell', () => {
    render(<LoginPage locale="en" onSuccess={vi.fn()} />);

    const continueButton = screen.getByRole('button', { name: /continue/i });
    expect(document.querySelector('.admin-shell')).toBeNull();
    expect(screen.getByRole('heading', { name: /login/i }).closest('.login-screen')).not.toBeNull();
    expect(screen.getByRole('heading', { name: /login/i }).closest('.login-card')).not.toBeNull();
    expect(screen.queryByText(/single-user local access/i)).toBeNull();
    expect(continueButton).toHaveClass('login-submit-button');
    expect(continueButton).toHaveClass('ui-button-primary');
  });

  it('requires a password before submitting', () => {
    const onSuccess = vi.fn();
    render(<LoginPage locale="en" onSuccess={onSuccess} />);

    fireEvent.click(screen.getByRole('button', { name: /continue/i }));

    expect(onSuccess).not.toHaveBeenCalled();
    expect(screen.getByText(/password is required/i)).toBeInTheDocument();
  });

  it('submits a trimmed password from the redesigned login card', () => {
    const onSuccess = vi.fn();
    render(<LoginPage locale="en" onSuccess={onSuccess} />);

    fireEvent.change(screen.getByLabelText(/password/i), {
      target: { value: '  my-secret  ' },
    });
    fireEvent.click(screen.getByRole('button', { name: /continue/i }));

    expect(onSuccess).toHaveBeenCalledWith('my-secret');
  });
});
