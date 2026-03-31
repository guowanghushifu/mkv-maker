import { useRef, useState } from 'react';
import type { DragEvent } from 'react';
import type { Draft, DraftTrack } from '../../api/types';
import { Button } from '../../components/Button';
import { getMessages, type Locale } from '../../i18n';
import { moveTrackRow, setExclusiveDefault, toggleTrackSelected } from './trackTable';

type TrackEditorPageProps = {
  locale?: Locale;
  draft: Draft;
  filenamePreview?: string;
  outputFilename?: string;
  onFilenameChange?: (value: string) => void;
  onChange: (next: Draft) => void;
  onBack?: () => void;
  onNext?: () => void;
};

type TrackGroup = 'audio' | 'subtitles';

function moveTrackByOffset(tracks: DraftTrack[], trackId: string, offset: -1 | 1): DraftTrack[] {
  const index = tracks.findIndex((track) => track.id === trackId);
  const nextIndex = index + offset;
  if (index < 0 || nextIndex < 0 || nextIndex >= tracks.length) {
    return tracks;
  }
  const next = [...tracks];
  const [moved] = next.splice(index, 1);
  next.splice(nextIndex, 0, moved);
  return next;
}

export function TrackEditorPage({
  locale = 'zh',
  draft,
  filenamePreview,
  outputFilename,
  onFilenameChange,
  onChange,
  onBack,
  onNext,
}: TrackEditorPageProps) {
  const text = getMessages(locale);
  const dragRef = useRef<{ group: TrackGroup; trackId: string } | null>(null);
  const [dropTarget, setDropTarget] = useState<{ group: TrackGroup; trackId: string } | null>(null);

  const updateVideoName = (name: string) => {
    onChange({ ...draft, video: { ...draft.video, name } });
  };

  const updateTitle = (title: string) => {
    onChange({ ...draft, title });
  };

  const updateAudio = (audio: DraftTrack[]) => {
    onChange({ ...draft, audio });
  };

  const updateSubtitles = (subtitles: DraftTrack[]) => {
    onChange({ ...draft, subtitles });
  };

  const updateAudioTrack = (trackId: string, updater: (track: DraftTrack) => DraftTrack) => {
    updateAudio(draft.audio.map((track) => (track.id === trackId ? updater(track) : track)));
  };

  const updateSubtitleTrack = (trackId: string, updater: (track: DraftTrack) => DraftTrack) => {
    updateSubtitles(draft.subtitles.map((track) => (track.id === trackId ? updater(track) : track)));
  };

  const handleDragStart = (event: DragEvent<HTMLButtonElement>, group: TrackGroup, trackId: string) => {
    dragRef.current = { group, trackId };
    event.dataTransfer.effectAllowed = 'move';
    event.dataTransfer.setData('text/plain', trackId);
  };

  const handleDragEnd = () => {
    dragRef.current = null;
    setDropTarget(null);
  };

  const handleDrop = (
    event: DragEvent<HTMLTableRowElement>,
    group: TrackGroup,
    targetTrackId: string,
  ) => {
    event.preventDefault();
    const sourceId =
      (dragRef.current?.group === group && dragRef.current.trackId) ||
      event.dataTransfer.getData('text/plain');
    setDropTarget(null);
    if (!sourceId || sourceId === targetTrackId) {
      return;
    }
    if (group === 'audio') {
      updateAudio(moveTrackRow(draft.audio, sourceId, targetTrackId));
      return;
    }
    updateSubtitles(moveTrackRow(draft.subtitles, sourceId, targetTrackId));
  };

  const handleKeyboardReorder = (group: TrackGroup, trackId: string, key: string) => {
    if (key !== 'ArrowUp' && key !== 'ArrowDown') {
      return;
    }
    const offset: -1 | 1 = key === 'ArrowUp' ? -1 : 1;
    if (group === 'audio') {
      updateAudio(moveTrackByOffset(draft.audio, trackId, offset));
      return;
    }
    updateSubtitles(moveTrackByOffset(draft.subtitles, trackId, offset));
  };

  const renderTrackTable = (group: TrackGroup, tracks: DraftTrack[]) => {
    const isAudioTable = group === 'audio';

    return (
      <div className="track-section-panel">
        <div className="track-table-wrap">
          <table className="track-editor-table">
            <colgroup>
              <col className="col-drag" />
              <col className="col-id" />
              <col className="col-track" />
              <col className="col-language" />
              {isAudioTable ? <col className="col-audio-format" /> : null}
              <col className="col-include" />
              <col className="col-default" />
            </colgroup>
            <thead>
              <tr>
                <th scope="col">{text.editor.columns.drag}</th>
                <th scope="col">{text.editor.columns.id}</th>
                <th scope="col">{text.editor.columns.track}</th>
                <th scope="col">{text.editor.columns.language}</th>
                {isAudioTable ? <th scope="col">{text.editor.columns.audioFormat}</th> : null}
                <th scope="col">{text.editor.columns.include}</th>
                <th scope="col">{text.editor.columns.default}</th>
              </tr>
            </thead>
            <tbody>
              {tracks.map((track) => (
                <tr
                  key={track.id}
                  className={[
                    track.selected ? 'is-selected' : 'is-muted',
                    dropTarget?.group === group && dropTarget.trackId === track.id ? 'is-drop-target' : '',
                  ]
                    .join(' ')
                    .trim()}
                  onDragEnter={() => setDropTarget({ group, trackId: track.id })}
                  onDragLeave={() => setDropTarget((current) => {
                    if (current?.group === group && current.trackId === track.id) {
                      return null;
                    }
                    return current;
                  })}
                  onDragOver={(event) => {
                    event.preventDefault();
                    setDropTarget({ group, trackId: track.id });
                    if (event.dataTransfer) {
                      event.dataTransfer.dropEffect = 'move';
                    }
                  }}
                  onDrop={(event) => handleDrop(event, group, track.id)}
                >
                  <td className="drag-cell">
                    <button
                      type="button"
                      className="drag-handle"
                      aria-label={text.editor.dragTrack(track.name)}
                      draggable
                      onDragStart={(event) => handleDragStart(event, group, track.id)}
                      onDragEnd={handleDragEnd}
                      onKeyDown={(event) => handleKeyboardReorder(group, track.id, event.key)}
                    >
                      ↕
                    </button>
                  </td>
                  <td className="track-id-cell">{track.id}</td>
                  <td>
                    <input
                      type="text"
                      className="track-name-input"
                      aria-label={text.editor.trackName(track.name)}
                      value={track.name}
                      onChange={(event) => {
                        if (group === 'audio') {
                          updateAudioTrack(track.id, (current) => ({ ...current, name: event.target.value }));
                          return;
                        }
                        updateSubtitleTrack(track.id, (current) => ({ ...current, name: event.target.value }));
                      }}
                    />
                  </td>
                  <td>
                    <input
                      type="text"
                      className="track-language-input"
                      aria-label={text.editor.language(track.name)}
                      value={track.language}
                      onChange={(event) => {
                        if (group === 'audio') {
                          updateAudioTrack(track.id, (current) => ({
                            ...current,
                            language: event.target.value,
                          }));
                          return;
                        }
                        updateSubtitleTrack(track.id, (current) => ({
                          ...current,
                          language: event.target.value,
                        }));
                      }}
                    />
                  </td>
                  {isAudioTable ? <td className="track-audio-format-cell">{track.codecLabel || ''}</td> : null}
                  <td>
                    <input
                      type="checkbox"
                      aria-label={text.editor.include(track.name)}
                      checked={track.selected}
                      onChange={() => {
                        if (group === 'audio') {
                          updateAudio(toggleTrackSelected(draft.audio, track.id));
                          return;
                        }
                        updateSubtitles(toggleTrackSelected(draft.subtitles, track.id));
                      }}
                    />
                  </td>
                  <td>
                    <input
                      type="checkbox"
                      aria-label={text.editor.default(track.name)}
                      checked={track.default}
                      disabled={!track.selected}
                      onChange={() => {
                        if (group === 'audio') {
                          updateAudio(setExclusiveDefault(draft.audio, track.id));
                          return;
                        }
                        updateSubtitles(setExclusiveDefault(draft.subtitles, track.id));
                      }}
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    );
  };

  return (
    <section className="panel page-panel editor-panel">
      <div className="panel-header">
        <div>
          <h2>{text.editor.title}</h2>
        </div>
      </div>

      <div className="editor-overview-grid">
        <div className="editor-overview-pair">
          <article className="editor-overview-card">
            <div className="stack editor-field-full">
              <label htmlFor="draft-title">{text.editor.titleLabel}</label>
              <input
                id="draft-title"
                type="text"
                value={draft.title || ''}
                onChange={(event) => updateTitle(event.target.value)}
              />
              <p className="editor-help-text">{text.editor.titleHint}</p>
            </div>
          </article>

          <article className="editor-overview-card">
            <div className="stack editor-field-full">
              <label htmlFor="video-track-name">{text.editor.videoTrackNameLabel}</label>
              <input
                id="video-track-name"
                type="text"
                value={draft.video.name}
                onChange={(event) => updateVideoName(event.target.value)}
              />
            </div>
            <p className="editor-meta-line">
              {text.editor.videoSourceAttributes}: {draft.video.codec} / {draft.video.resolution}
              {draft.video.hdrType ? ` / ${draft.video.hdrType}` : ''}
            </p>
          </article>
        </div>

        {typeof filenamePreview === 'string' ? (
          <article className="editor-overview-card editor-overview-card-wide">
            <p>
              <strong>{text.editor.liveFilenamePreview}:</strong> {filenamePreview}
            </p>
            {onFilenameChange ? (
              <div className="stack">
                <label htmlFor="output-filename">{text.editor.outputFilename}</label>
                <input
                  id="output-filename"
                  type="text"
                  value={outputFilename || ''}
                  onChange={(event) => onFilenameChange(event.target.value)}
                />
              </div>
            ) : null}
          </article>
        ) : null}
      </div>

      <section className="editor-track-section">
        <div className="section-heading">
          <h3>{text.editor.audioHeading}</h3>
        </div>
        {renderTrackTable('audio', draft.audio)}
      </section>

      <section className="editor-track-section">
        <div className="section-heading">
          <h3>{text.editor.subtitlesHeading}</h3>
        </div>
        {draft.subtitles.length === 0 ? (
          <p className="muted-text">{text.editor.noSubtitles}</p>
        ) : (
          renderTrackTable('subtitles', draft.subtitles)
        )}
      </section>

      {onBack || onNext ? (
        <div className="row editor-actions">
          {onBack ? (
            <Button variant="subtle" onClick={onBack}>
              {text.editor.backButton}
            </Button>
          ) : null}
          {onNext ? (
            <Button onClick={onNext}>
              {text.editor.nextButton}
            </Button>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}
