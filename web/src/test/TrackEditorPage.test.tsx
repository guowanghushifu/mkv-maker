import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { TrackEditorPage } from '../features/draft/TrackEditorPage';

describe('TrackEditorPage', () => {
  it('moves a selected audio track upward in the export order', () => {
    const onChange = vi.fn();
    render(
      <TrackEditorPage
        draft={{
          video: { name: 'Main Video', codec: 'HEVC', resolution: '2160p', hdrType: 'HDR.DV' },
          audio: [
            { id: 'a1', name: 'English Atmos', language: 'eng', selected: true, default: true },
            { id: 'a2', name: 'Commentary', language: 'eng', selected: true, default: false },
          ],
          subtitles: [],
        }}
        onChange={onChange}
      />,
    );

    fireEvent.click(screen.getByRole('button', { name: /move commentary up/i }));
    expect(onChange).toHaveBeenCalled();
  });
});
