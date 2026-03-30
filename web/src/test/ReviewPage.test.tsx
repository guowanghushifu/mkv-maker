import { render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { createApiClient } from '../api/client';
import { ReviewPage } from '../features/review/ReviewPage';

describe('ReviewPage', () => {
  it('renders progress percentage, progress bar, and formatted command preview', () => {
    const source = {
      id: 'disc-1',
      name: 'Nightcrawler Disc',
      path: '/bd_input/Nightcrawler/BDMV',
      type: 'bdmv',
      size: 1,
      modifiedAt: '2026-03-30T00:00:00Z',
    } as const;
    const bdinfo = {
      playlistName: '00003.MPLS',
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
        startDisabled={false}
        submitError={null}
        currentJob={{
          id: 'job-123',
          sourceName: 'Nightcrawler Disc',
          outputName: 'Nightcrawler - 2160p.mkv',
          outputPath: '/remux/Nightcrawler - 2160p.mkv',
          playlistName: '00003.MPLS',
          createdAt: '2026-03-30T00:00:00Z',
          status: 'running',
          progressPercent: 42,
          commandPreview: 'mkvmerge\n  --output\n  /remux/Nightcrawler - 2160p.mkv',
        }}
        currentLog="[2026-03-30T00:00:01Z] Progress: 42%"
        onBack={() => {}}
        onSubmit={() => {}}
      />
    );

    expect(screen.getByText('42%')).toBeInTheDocument();
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '42');
    expect(screen.getByText(/mkvmerge/i)).toBeInTheDocument();
    expect(screen.getByText(/--output/i)).toBeInTheDocument();
    expect(screen.getByText(/mkvmerge/i).closest('pre')).toHaveClass('scroll-panel');
    expect(screen.getByText(/\[2026-03-30T00:00:01Z\] Progress: 42%/i)).toHaveClass('scroll-panel');
    expect(screen.getByRole('button', { name: /start next remux/i }).closest('.review-actions-secondary')).not.toBeNull();
  });

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
        startDisabled={false}
        submitError={null}
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
    expect(screen.getByRole('button', { name: /back/i }).compareDocumentPosition(screen.getByText(/current remux/i))).toBe(
      Node.DOCUMENT_POSITION_FOLLOWING
    );
  });

  it('shows 0 percent progress when current job has no progressPercent yet', () => {
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
        startDisabled={false}
        submitError={null}
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

    expect(screen.getByText('0%')).toBeInTheDocument();
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '0');
  });

  it('disables start while a remux is currently running', () => {
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
        startDisabled
        submitError="A remux is already running. Please wait for it to finish."
        currentJob={{
          id: 'job-123',
          sourceName: 'Nightcrawler Disc',
          outputName: 'Nightcrawler - 2160p.mkv',
          outputPath: '/remux/Nightcrawler - 2160p.mkv',
          playlistName: '00800.MPLS',
          createdAt: '2026-03-29T12:00:00Z',
          status: 'running',
        }}
        currentLog=""
        onBack={() => {}}
        onSubmit={() => {}}
      />
    );

    expect(screen.getByRole('button', { name: /start remux/i })).toBeDisabled();
    expect(screen.getByText(/already running/i)).toBeInTheDocument();
  });
});

describe('api current job fallbacks', () => {
  it('returns null and empty log for 404 current job endpoints', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith('/api/jobs/current')) {
        return new Response('', { status: 404 });
      }
      if (url.endsWith('/api/jobs/current/log')) {
        return new Response('', { status: 404 });
      }
      return new Response('', { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const client = createApiClient('/api');
    await expect(client.currentJob('session')).resolves.toBeNull();
    await expect(client.currentJobLog('session')).resolves.toBe('');

    vi.unstubAllGlobals();
  });
});
