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

  it('updates editable track fields for names and languages', () => {
    const onChange = vi.fn();
    render(
      <TrackEditorPage
        draft={{
          title: 'Demo Title',
          video: { name: 'Main Video', codec: 'HEVC', resolution: '2160p', hdrType: 'HDR.DV' },
          audio: [
            { id: 'a1', name: 'English Atmos', language: 'eng', selected: true, default: true },
          ],
          subtitles: [{ id: 's1', name: 'English PGS', language: 'eng', selected: true, default: false }],
        }}
        onChange={onChange}
      />,
    );

    fireEvent.change(screen.getByLabelText('Video track name'), { target: { value: 'Feature Video' } });
    fireEvent.change(screen.getByLabelText('Language', { selector: '#audio-lang-a1' }), {
      target: { value: 'jpn' },
    });
    fireEvent.change(screen.getByLabelText('Language', { selector: '#subtitle-lang-s1' }), {
      target: { value: 'spa' },
    });

    expect(onChange).toHaveBeenCalled();
  });

  it('allows subtitle default selection and selected-track reordering', () => {
    const onChange = vi.fn();
    render(
      <TrackEditorPage
        draft={{
          video: { name: 'Main Video', codec: 'HEVC', resolution: '2160p', hdrType: 'HDR.DV' },
          audio: [],
          subtitles: [
            { id: 's1', name: 'English PGS', language: 'eng', selected: true, default: false },
            { id: 's2', name: 'Commentary Subs', language: 'eng', selected: true, default: false },
          ],
        }}
        onChange={onChange}
      />,
    );

    fireEvent.click(screen.getAllByRole('radio', { name: /default/i })[0]);
    fireEvent.click(screen.getByRole('button', { name: /move commentary subs up/i }));

    expect(onChange).toHaveBeenCalled();
  });
});
