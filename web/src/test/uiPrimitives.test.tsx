import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { Button } from '../components/Button';
import { SummaryCard } from '../components/SummaryCard';

describe('UI primitives', () => {
  it('renders button variants with shared base classes', () => {
    render(
      <>
        <Button onClick={vi.fn()}>Primary</Button>
        <Button variant="subtle">Subtle</Button>
      </>
    );

    expect(screen.getByRole('button', { name: 'Primary' })).toHaveClass('ui-button', 'ui-button-primary');
    expect(screen.getByRole('button', { name: 'Subtle' })).toHaveClass('ui-button', 'ui-button-subtle');
  });

  it('renders summary card labels and values with supplied shell class names', () => {
    render(
      <SummaryCard
        className="review-summary-card"
        labelClassName="summary-label"
        valueClassName="summary-value"
        label="Source"
        value="Nightcrawler Disc"
      />
    );

    expect(screen.getByText('Source').closest('.review-summary-card')).not.toBeNull();
    expect(screen.getByText('Nightcrawler Disc')).toHaveClass('summary-value');
  });
});
