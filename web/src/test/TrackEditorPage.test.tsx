import { fireEvent, render, screen, within } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { TrackEditorPage } from '../features/draft/TrackEditorPage';

function createDraft() {
  return {
    title: 'Demo Title',
    video: { name: 'Main Video', codec: 'HEVC', resolution: '2160p', hdrType: 'DV.HDR' },
    audio: [
      {
        id: 'A1',
        sourceIndex: 0,
        name: 'English Atmos',
        language: 'eng',
        codecLabel: 'TrueHD.7.1.Atmos',
        selected: true,
        default: true,
      },
      {
        id: 'A2',
        sourceIndex: 1,
        name: 'Commentary',
        language: 'eng',
        codecLabel: 'DTS-HD.MA.7.1',
        selected: true,
        default: false,
      },
    ],
    subtitles: [
      { id: 'S1', sourceIndex: 0, name: 'English PGS', language: 'eng', selected: true, default: true },
      { id: 'S2', sourceIndex: 1, name: 'Signs', language: 'eng', selected: true, default: false },
    ],
    makemkv: {
      playlistName: '00800.MPLS',
      titleId: 0,
      audio: [
        {
          id: 'A1',
          sourceIndex: 0,
          name: 'English Atmos',
          language: 'eng',
          codecLabel: 'TrueHD.7.1.Atmos',
          selected: true,
          default: true,
        },
        {
          id: 'A2',
          sourceIndex: 1,
          name: 'Commentary',
          language: 'eng',
          codecLabel: 'DTS-HD.MA.7.1',
          selected: true,
          default: false,
        },
      ],
      subtitles: [
        { id: 'S1', sourceIndex: 0, name: 'English PGS', language: 'eng', selected: true, default: true },
        { id: 'S2', sourceIndex: 1, name: 'Signs', language: 'eng', selected: true, default: false },
      ],
    },
  };
}

function createRecommendationDraft() {
  return {
    ...createDraft(),
    audio: [
      {
        id: 'A1',
        sourceIndex: 0,
        name: 'English Atmos',
        language: 'eng',
        codecLabel: 'TrueHD.7.1.Atmos',
        selected: false,
        default: false,
      },
      {
        id: 'A2',
        sourceIndex: 1,
        name: 'Mandarin',
        language: 'zho',
        codecLabel: 'DTS-HD.MA.5.1',
        selected: false,
        default: false,
      },
      {
        id: 'A3',
        sourceIndex: 2,
        name: 'French',
        language: 'fra',
        codecLabel: 'AC3.5.1',
        selected: true,
        default: true,
      },
      {
        id: 'A4',
        sourceIndex: 3,
        name: 'Cantonese',
        language: 'zh-Hans',
        codecLabel: 'AAC.2.0',
        selected: false,
        default: false,
      },
    ],
    subtitles: [
      { id: 'S1', sourceIndex: 0, name: 'English PGS', language: 'eng', selected: false, default: false },
      { id: 'S2', sourceIndex: 1, name: 'Chinese PGS', language: 'chi', selected: false, default: false },
      { id: 'S3', sourceIndex: 2, name: 'French PGS', language: 'fra', selected: true, default: true },
      { id: 'S4', sourceIndex: 3, name: 'Commentary', language: 'zh-cn', selected: false, default: false },
    ],
  };
}

describe('TrackEditorPage', () => {
  it('renders the editor inside workspace and table cards', () => {
    const { container } = render(<TrackEditorPage locale="en" draft={createDraft()} onChange={vi.fn()} />);

    expect(container.querySelector('.workspace-card.editor-workspace')).not.toBeNull();
    expect(container.querySelectorAll('.editor-section-card').length).toBeGreaterThan(0);
    expect(container.querySelectorAll('.track-section-panel').length).toBeGreaterThan(1);
  });

  it('shows audio format only in the audio table and keeps subtitle headers unchanged', () => {
    const { container } = render(<TrackEditorPage locale="en" draft={createDraft()} onChange={vi.fn()} />);

    const [audioTable, subtitleTable] = screen.getAllByRole('table');
    expect(within(audioTable).getAllByRole('columnheader').map((header) => header.textContent?.trim())).toEqual([
      'Move',
      'ID',
      'Track',
      'Language',
      'Audio Format',
      'Include',
      'Default',
    ]);
    expect(
      within(subtitleTable).getAllByRole('columnheader').map((header) => header.textContent?.trim()),
    ).toEqual(['Move', 'ID', 'Track', 'Language', 'Include', 'Default']);

    expect(screen.getByText('A1')).toBeInTheDocument();
    expect(screen.getByText('A2')).toBeInTheDocument();
    expect(screen.getByText('TrueHD.7.1.Atmos')).toBeInTheDocument();
    expect(screen.getByText('DTS-HD.MA.7.1')).toBeInTheDocument();
    expect(screen.queryByRole('columnheader', { name: 'Details' })).not.toBeInTheDocument();
    expect(container.querySelector('.track-editor-table .col-drag')).toHaveClass('col-drag');
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
    expect(screen.getByLabelText(/^title$/i).closest('.editor-field-full')).not.toBeNull();
    expect(screen.getByLabelText(/video track name/i).closest('.editor-field-full')).not.toBeNull();
    expect(screen.getByLabelText(/output filename/i).closest('.editor-overview-card-wide')).not.toBeNull();
    expect(screen.getByText(/video source attributes: hevc \/ 2160p \/ dv\.hdr/i).closest('.editor-overview-card')).not.toBeNull();
    expect(
      screen.getByText(/recommended: name \+ year, example: nightcrawler\.2014/i).closest('.editor-meta-line')
    ).not.toBeNull();
  });

  it('renders include and default controls as switches', () => {
    render(<TrackEditorPage locale="en" draft={createDraft()} onChange={vi.fn()} />);

    expect(screen.getByRole('switch', { name: /include english atmos/i })).toBeInTheDocument();
    expect(screen.getByRole('switch', { name: /default commentary/i })).toBeInTheDocument();
    expect(screen.queryByRole('checkbox', { name: /include english atmos/i })).not.toBeInTheDocument();
  });

  it('clears the previous default audio track when a new default switch is turned on', () => {
    const onChange = vi.fn();
    render(<TrackEditorPage locale="en" draft={createDraft()} onChange={onChange} />);

    fireEvent.click(screen.getByRole('switch', { name: /default commentary/i }));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        audio: [
          expect.objectContaining({ id: 'A1', default: false }),
          expect.objectContaining({ id: 'A2', default: true }),
        ],
      }),
    );
  });

  it('clears default when deselecting a defaulted subtitle track', () => {
    const onChange = vi.fn();
    render(<TrackEditorPage locale="en" draft={createDraft()} onChange={onChange} />);

    fireEvent.click(screen.getByRole('switch', { name: /include english pgs/i }));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        subtitles: [
          expect.objectContaining({ id: 'S1', selected: false, default: false }),
          expect.objectContaining({ id: 'S2', selected: true, default: false }),
        ],
      }),
    );
  });

  it('keeps a default switch disabled when the track is not included', () => {
    const onChange = vi.fn();
    render(
      <TrackEditorPage
        locale="en"
        draft={{
          ...createDraft(),
          subtitles: [
            { id: 'S1', sourceIndex: 0, name: 'English PGS', language: 'eng', selected: false, default: false },
            { id: 'S2', sourceIndex: 1, name: 'Signs', language: 'eng', selected: true, default: true },
          ],
        }}
        onChange={onChange}
      />,
    );

    const disabledDefault = screen.getByRole('switch', { name: /default english pgs/i });
    expect(disabledDefault).toBeDisabled();

    fireEvent.click(disabledDefault);
    expect(onChange).not.toHaveBeenCalled();
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
          expect.objectContaining({ id: 'A2' }),
          expect.objectContaining({ id: 'A1' }),
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
            { id: 'A1', sourceIndex: 0, name: 'English Atmos', language: 'eng', selected: true, default: true },
            { id: 'A2', sourceIndex: 1, name: 'Commentary', language: 'eng', selected: true, default: false },
            { id: 'A3', sourceIndex: 2, name: 'French', language: 'fra', selected: true, default: false },
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
          expect.objectContaining({ id: 'A2' }),
          expect.objectContaining({ id: 'A1' }),
          expect.objectContaining({ id: 'A3' }),
        ],
      }),
    );
  });

  it('supports keyboard reorder from drag handle controls', () => {
    const onChange = vi.fn();
    render(<TrackEditorPage locale="en" draft={createDraft()} onChange={onChange} />);

    expect(screen.getByRole('button', { name: /drag commentary/i })).toHaveTextContent('↕');
    fireEvent.keyDown(screen.getByRole('button', { name: /drag commentary/i }), { key: 'ArrowUp' });

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        audio: [
          expect.objectContaining({ id: 'A2' }),
          expect.objectContaining({ id: 'A1' }),
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

  it('resets audio selection from the recommendation button', () => {
    const onChange = vi.fn();
    render(<TrackEditorPage locale="zh" draft={createRecommendationDraft()} onChange={onChange} />);

    fireEvent.click(screen.getByRole('button', { name: '猜你喜欢音频' }));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        audio: [
          expect.objectContaining({ id: 'A1', selected: true, default: true }),
          expect.objectContaining({ id: 'A2', selected: true, default: false }),
          expect.objectContaining({ id: 'A3', selected: false, default: false }),
          expect.objectContaining({ id: 'A4', selected: true, default: false }),
        ],
      }),
    );
  });

  it('resets subtitle selection from the recommendation button', () => {
    const onChange = vi.fn();
    render(<TrackEditorPage locale="zh" draft={createRecommendationDraft()} onChange={onChange} />);

    fireEvent.click(screen.getByRole('button', { name: '猜你喜欢字幕' }));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenLastCalledWith(
      expect.objectContaining({
        subtitles: [
          expect.objectContaining({ id: 'S1', selected: true, default: true }),
          expect.objectContaining({ id: 'S2', selected: true, default: false }),
          expect.objectContaining({ id: 'S3', selected: false, default: false }),
          expect.objectContaining({ id: 'S4', selected: true, default: false }),
        ],
      }),
    );
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
