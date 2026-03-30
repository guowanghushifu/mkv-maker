import { afterEach, describe, expect, it, vi } from 'vitest';
import { buildFilenamePreview, createApiClient } from '../api/client';

describe('buildFilenamePreview', () => {
  it('does not fall back to a descriptive default-audio display name', () => {
    const filename = buildFilenamePreview(
      {
        title: 'Nightcrawler',
        dvMergeEnabled: true,
        video: {
          name: 'Main Video',
          codec: 'HEVC',
          resolution: '2160p',
          hdrType: 'HDR.DV',
        },
        audio: [
          {
            id: 'a1',
            name: '英文次世代全景声',
            language: 'eng',
            selected: true,
            default: true,
          },
        ],
        subtitles: [],
      },
      'Nightcrawler'
    );

    expect(filename).toBe('Nightcrawler - 2160p.BluRay.HDR.DV.HEVC.UnknownAudio.mkv');
  });
});

describe('createApiClient currentJob', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('preserves command preview and progress percent from current job payload', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
      const url = String(input);
      if (url.endsWith('/api/jobs/current')) {
        return new Response(
          JSON.stringify({
            id: 'job-1',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00003.MPLS',
            createdAt: '2026-03-30T00:00:00Z',
            status: 'running',
            progressPercent: 37,
            commandPreview: 'mkvmerge\n  --output\n  /remux/Nightcrawler - 2160p.mkv',
          }),
          {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          }
        );
      }
      return new Response('', { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const client = createApiClient('/api');
    await expect(client.currentJob('session')).resolves.toMatchObject({
      progressPercent: 37,
      commandPreview: 'mkvmerge\n  --output\n  /remux/Nightcrawler - 2160p.mkv',
    });
  });
});
