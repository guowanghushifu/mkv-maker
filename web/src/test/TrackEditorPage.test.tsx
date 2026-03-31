import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { TrackEditorPage } from '../features/draft/TrackEditorPage';

function createDraft() {
  return {
    title: 'Demo Title',
    video: { name: 'Main Video', codec: 'HEVC', resolution: '2160p', hdrType: 'DV.HDR' },
    audio: [
      {
        id: 'a1',
        name: 'English Atmos',
        language: 'eng',
        codecLabel: 'TrueHD.7.1.Atmos',
        selected: true,
        default: true,
      },
      {
        id: 'a2',
        name: 'Commentary',
        language: 'eng',
        codecLabel: 'DTS-HD.MA.7.1',
        selected: true,
        default: false,
      },
    ],
    subtitles: [
      { id: 's1', name: 'English PGS', language: 'eng', selected: true, default: true },
      { id: 's2', name: 'Signs', language: 'eng', selected: true, default: false },
    ],
  };
}

describe('TrackEditorPage', () => {
  it('shows audio format only in the audio table and keeps subtitle headers unchanged', () => {
    render(<TrackEditorPage locale="en" draft={createDraft()} onChange={vi.fn()} />);

    const [audioTable, subtitleTable] = screen.getAllByRole('table');
    expect(within(audioTable).getAllByRole('columnheader').map((header) => header.textContent?.trim())).toEqual([
      '',
      'ID',
      'Track',
      'Language',
      'Audio Format',
      'Include',
      'Default',
    ]);
    expect(
      within(subtitleTable).getAllByRole('columnheader').map((header) => header.textContent?.trim()),
    ).toEqual(['', 'ID', 'Track', 'Language', 'Include', 'Default']);

    expect(screen.getByText('a1')).toBeInTheDocument();
    expect(screen.getByText('a2')).toBeInTheDocument();
    expect(screen.getByText('TrueHD.7.1.Atmos')).toBeInTheDocument();
    expect(screen.getByText('DTS-HD.MA.7.1')).toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: 'Details' })).not.toBeInTheDocument();
  });

  it('renders dolby vision video attributes directly from the unified DV.HDR hdrType', () => {
    render(<TrackEditorPage locale="en" draft={createDraft()} onChange={vi.fn()} />);

    expect(screen.getByText(/video source attributes: hevc \/ 2160p \/ dv\.hdr/i)).toBeInTheDocument();
    expect(screen.queryByText(/video source attributes: hevc \/ 2160p \/ hdr\.dv/i)).not.toBeInTheDocument();
  });

  it('renders overview cards for title, video source attributes, and filename preview', () => {
    render(
      <TrackEditorPage
        locale="en"
        draft={createDraft()}
        filenamePreview="Demo.Title.2160p.DV.mkv"
        outputFilename="Demo.Title.2160p.DV.mkv"
        onFilenameChange={vi.fn()}
        onChange={vi.fn()}
      />
    );

    expect(screen.getByRole('heading', { name: /track editor/i }).closest('.page-panel')).not.toBeNull();
    expect(screen.getByLabelText(/title/i).closest('.editor-overview-pair')).not.toBeNull();
    expect(screen.getByLabelText(/video track name/i).closest('.editor-overview-pair')).not.toBeNull();
    expect(screen.getByLabelText(/output filename/i).closest('.editor-overview-card-wide')).not.toBeNull();
    expect(screen.getByText(/video source attributes: hevc \/ 2160p \/ dv\.hdr/i).closest('.editor-overview-card')).not.toBeNull();
  });

  it('clears the previous default audio track when a new default is checked', () => {
    const onChange = vi.fn();
    render(<TrackEditorPage locale="en" draft={createDraft()} onChange={onChange} />);

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
    render(<TrackEditorPage locale="en" draft={createDraft()} onChange={onChange} />);

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
    render(<TrackEditorPage locale="en" draft={createDraft()} onChange={onChange} />);

    const source = screen.getByRole('button', { name: /drag commentary/i });
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

  it('reorders audio tracks downward by inserting before the drop target row', () => {
    const onChange = vi.fn();
    render(
      <TrackEditorPage
        locale="en"
        draft={{
          ...createDraft(),
          audio: [
            { id: 'a1', name: 'English Atmos', language: 'eng', selected: true, default: true },
            { id: 'a2', name: 'Commentary', language: 'eng', selected: true, default: false },
            { id: 'a3', name: 'French', language: 'fra', selected: true, default: false },
          ],
        }}
        onChange={onChange}
      />,
    );

    const source = screen.getByRole('button', { name: /drag english atmos/i });
    const target = screen.getByRole('row', { name: /french/i });
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
          expect.objectContaining({ id: 'a3' }),
        ],
      }),
    );
  });

  it('supports keyboard reorder from drag handle controls', () => {
    const onChange = vi.fn();
    render(<TrackEditorPage locale="en" draft={createDraft()} onChange={onChange} />);

    fireEvent.keyDown(screen.getByRole('button', { name: /drag commentary/i }), { key: 'ArrowUp' });

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

  it('shows drop target highlight during drag over', () => {
    const onChange = vi.fn();
    render(<TrackEditorPage locale="en" draft={createDraft()} onChange={onChange} />);

    const source = screen.getByRole('button', { name: /drag commentary/i });
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
    fireEvent.dragEnter(target);
    expect(target.className).toContain('is-drop-target');
    fireEvent.drop(target, { dataTransfer });
    expect(target.className).not.toContain('is-drop-target');
  });

  it('supports inline name and language edits', () => {
    const onChange = vi.fn();
    render(<TrackEditorPage locale="en" draft={createDraft()} onChange={onChange} />);

    fireEvent.change(screen.getByLabelText('Video track name'), { target: { value: 'Feature Video' } });
    fireEvent.change(screen.getByLabelText('Track name English Atmos'), { target: { value: 'English 5.1' } });
    fireEvent.change(screen.getByLabelText('Language English Atmos'), { target: { value: 'jpn' } });

    expect(onChange).toHaveBeenCalled();
  });

  it('renders the bottom editor actions in a dedicated spaced container', () => {
    render(
      <TrackEditorPage
        locale="en"
        draft={createDraft()}
        onChange={vi.fn()}
        onBack={vi.fn()}
        onNext={vi.fn()}
      />,
    );

    expect(screen.getByRole('button', { name: /back/i }).closest('.editor-actions')).not.toBeNull();
    expect(screen.getByRole('button', { name: /continue to review/i }).closest('.editor-actions')).not.toBeNull();
  });

  it('wraps audio and subtitle tables in dedicated track section panels', () => {
    render(<TrackEditorPage locale="en" draft={createDraft()} onChange={vi.fn()} />);

    expect(screen.getByRole('heading', { name: /^audio$/i }).closest('.editor-track-section')).not.toBeNull();
    expect(screen.getByRole('heading', { name: /^subtitles$/i }).closest('.editor-track-section')).not.toBeNull();
    expect(screen.getAllByRole('table')[0].closest('.track-section-panel')).not.toBeNull();
    expect(screen.getAllByRole('table')[1].closest('.track-section-panel')).not.toBeNull();
  });
});
