import { fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { TrackEditorPage } from '../features/draft/TrackEditorPage';

function createDraft() {
  return {
    title: 'Demo Title',
    video: { name: 'Main Video', codec: 'HEVC', resolution: '2160p', hdrType: 'HDR.DV' },
    audio: [
      { id: 'a1', name: 'English Atmos', language: 'eng', selected: true, default: true },
      { id: 'a2', name: 'Commentary', language: 'eng', selected: true, default: false },
    ],
    subtitles: [
      { id: 's1', name: 'English PGS', language: 'eng', selected: true, default: true },
      { id: 's2', name: 'Signs', language: 'eng', selected: true, default: false },
    ],
  };
}

describe('TrackEditorPage', () => {
  it('clears the previous default audio track when a new default is checked', () => {
    const onChange = vi.fn();
    render(<TrackEditorPage draft={createDraft()} onChange={onChange} />);

    fireEvent.click(screen.getByRole('checkbox', { name: /default commentary/i }));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        audio: [
          expect.objectContaining({ id: 'a1', default: false }),
          expect.objectContaining({ id: 'a2', default: true }),
        ],
      }),
    );
  });

  it('clears default when deselecting a defaulted subtitle track', () => {
    const onChange = vi.fn();
    render(<TrackEditorPage draft={createDraft()} onChange={onChange} />);

    fireEvent.click(screen.getByRole('checkbox', { name: /include english pgs/i }));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        subtitles: [
          expect.objectContaining({ id: 's1', selected: false, default: false }),
          expect.objectContaining({ id: 's2', selected: true, default: false }),
        ],
      }),
    );
  });

  it('reorders audio tracks via drag and drop', () => {
    const onChange = vi.fn();
    render(<TrackEditorPage draft={createDraft()} onChange={onChange} />);

    const source = screen.getByRole('row', { name: /commentary/i });
    const target = screen.getByRole('row', { name: /english atmos/i });
    const store = new Map<string, string>();
    const dataTransfer = {
      effectAllowed: '',
      setData: (type: string, value: string) => {
        store.set(type, value);
      },
      getData: (type: string) => store.get(type) || '',
    };

    fireEvent.dragStart(source, { dataTransfer });
    fireEvent.dragOver(target);
    fireEvent.drop(target, { dataTransfer });

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        audio: [
          expect.objectContaining({ id: 'a2' }),
          expect.objectContaining({ id: 'a1' }),
        ],
      }),
    );
  });

  it('supports inline name and language edits', () => {
    const onChange = vi.fn();
    render(<TrackEditorPage draft={createDraft()} onChange={onChange} />);

    fireEvent.change(screen.getByLabelText('Video track name'), { target: { value: 'Feature Video' } });
    fireEvent.change(screen.getByLabelText('Track name English Atmos'), { target: { value: 'English 5.1' } });
    fireEvent.change(screen.getByLabelText('Language English Atmos'), { target: { value: 'jpn' } });

    expect(onChange).toHaveBeenCalled();
  });
});
