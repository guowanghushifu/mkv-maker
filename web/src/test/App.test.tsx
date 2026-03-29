import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import App from '../App';

describe('App', () => {
  it('renders the application shell title', () => {
    render(<App />);
    expect(screen.getByRole('heading', { name: /MKV Remux Tool/i })).toBeInTheDocument();
    expect(screen.getByText('Review')).toBeInTheDocument();
    expect(screen.queryByText('Jobs')).not.toBeInTheDocument();
  });
});
