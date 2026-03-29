import { useRef, useState } from 'react';
import type { DragEvent } from 'react';
import type { Draft, DraftTrack } from '../../api/types';
import { moveTrackRow, setExclusiveDefault, toggleTrackSelected } from './trackTable';

type TrackEditorPageProps = {
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
  draft,
  filenamePreview,
  outputFilename,
  onFilenameChange,
  onChange,
  onBack,
  onNext,
}: TrackEditorPageProps) {
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

  const renderTrackTable = (group: TrackGroup, tracks: DraftTrack[]) => (
    <div className="track-table-wrap">
      <table className="track-editor-table">
        <thead>
          <tr>
            <th scope="col" aria-label="Drag" />
            <th scope="col">Include</th>
            <th scope="col">Track</th>
            <th scope="col">Language</th>
            <th scope="col">Default</th>
            <th scope="col">Details</th>
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
                  aria-label={`Drag ${track.name}`}
                  draggable
                  onDragStart={(event) => handleDragStart(event, group, track.id)}
                  onDragEnd={handleDragEnd}
                  onKeyDown={(event) => handleKeyboardReorder(group, track.id, event.key)}
                >
                  ⋮⋮
                </button>
              </td>
              <td>
                <input
                  type="checkbox"
                  aria-label={`Include ${track.name}`}
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
                  type="text"
                  aria-label={`Track name ${track.name}`}
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
                  aria-label={`Language ${track.name}`}
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
              <td>
                <input
                  type="checkbox"
                  aria-label={`Default ${track.name}`}
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
              <td>
                <span className="track-detail-chip">
                  {track.codecLabel || (group === 'audio' ? 'Audio track' : 'Subtitle track')}
                </span>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );

  return (
    <section className="panel">
      <h2>Track Editor</h2>

      <div className="stack">
        <label htmlFor="draft-title">Title</label>
        <input
          id="draft-title"
          type="text"
          value={draft.title || ''}
          onChange={(event) => updateTitle(event.target.value)}
        />
        <label htmlFor="video-track-name">Video track name</label>
        <input
          id="video-track-name"
          type="text"
          value={draft.video.name}
          onChange={(event) => updateVideoName(event.target.value)}
        />
      </div>

      <p>
        Video source attributes: {draft.video.codec} / {draft.video.resolution}
        {draft.video.hdrType ? ` / ${draft.video.hdrType}` : ''}
      </p>

      {typeof filenamePreview === 'string' ? (
        <div className="info-box">
          <p>
            <strong>Live filename preview:</strong> {filenamePreview}
          </p>
          {onFilenameChange ? (
            <>
              <label htmlFor="output-filename">Output filename</label>
              <input
                id="output-filename"
                type="text"
                value={outputFilename || ''}
                onChange={(event) => onFilenameChange(event.target.value)}
              />
            </>
          ) : null}
        </div>
      ) : null}

      <h3>Audio</h3>
      {renderTrackTable('audio', draft.audio)}

      <h3>Subtitles</h3>
      {draft.subtitles.length === 0 ? (
        <p className="muted-text">No subtitles found in this draft.</p>
      ) : (
        renderTrackTable('subtitles', draft.subtitles)
      )}

      {onBack || onNext ? (
        <div className="row">
          {onBack ? (
            <button type="button" onClick={onBack}>
              Back
            </button>
          ) : null}
          {onNext ? (
            <button type="button" onClick={onNext}>
              Continue to Review
            </button>
          ) : null}
        </div>
      ) : null}
    </section>
  );
}
