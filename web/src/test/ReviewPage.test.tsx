import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { ReviewPage } from '../features/review/ReviewPage';

describe('ReviewPage', () => {
  it('renders the current remux panel when a task is present', () => {
    const source = {
      id: 'disc-1',
      name: 'Nightcrawler Disc',
      path: '/bd_input/Nightcrawler/BDMV',
      type: 'bdmv',
      size: 1,
      modifiedAt: '2026-03-29T12:00:00Z',
    } as const;
    const bdinfo = {
      playlistName: '00800.MPLS',
      rawText: 'PLAYLIST REPORT',
      audioLabels: [],
      subtitleLabels: [],
    } as const;
    const draft = {
      title: 'Nightcrawler',
      outputDir: '/remux',
      dvMergeEnabled: true,
      video: { name: 'Main Video', codec: 'HEVC', resolution: '2160p', hdrType: 'HDR.DV' },
      audio: [],
      subtitles: [],
    } as const;

    render(
      <ReviewPage
        source={source}
        bdinfo={bdinfo}
        draft={draft}
        outputFilename="Nightcrawler - 2160p.mkv"
        outputPath="/remux/Nightcrawler - 2160p.mkv"
        submitting={false}
        currentJob={{
          id: 'job-123',
          sourceName: 'Nightcrawler Disc',
          outputName: 'Nightcrawler - 2160p.mkv',
          outputPath: '/remux/Nightcrawler - 2160p.mkv',
          playlistName: '00800.MPLS',
          createdAt: '2026-03-29T12:00:00Z',
          status: 'running',
        }}
        currentLog="[2026-03-29T12:00:00Z] remux started"
        onBack={() => {}}
        onSubmit={() => {}}
      />
    );

    expect(screen.getByText(/current remux/i)).toBeInTheDocument();
    expect(screen.getByText(/running/i)).toBeInTheDocument();
    expect(screen.getByText(/remux started/i)).toBeInTheDocument();
  });
});
