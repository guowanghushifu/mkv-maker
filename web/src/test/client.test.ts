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
          hdrType: 'DV.HDR',
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

    expect(filename).toBe('Nightcrawler - 2160p.BluRay.DV.HDR.HEVC.UnknownAudio.mkv');
  });

  it('preserves ascii parentheses and rewrites underscores to dots', () => {
    const filename = buildFilenamePreview(
      {
        title: 'Alien_(1979)',
        video: {
          name: 'Main Video',
          codec: 'HEVC',
          resolution: '2160p',
          hdrType: 'HDR',
        },
        audio: [
          {
            id: 'a1',
            name: 'English',
            language: 'eng',
            codecLabel: 'DTS_HD.MA.5.1',
            selected: true,
            default: true,
          },
        ],
        subtitles: [],
      },
      'Alien'
    );

    expect(filename).toBe('Alien.(1979) - 2160p.BluRay.HDR.HEVC.DTS.HD.MA.5.1.mkv');
  });
});

describe('createApiClient currentJob', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('does not send Authorization headers for protected requests', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL, init?: RequestInit) => {
      const headers = new Headers(init?.headers);
      expect(headers.has('Authorization')).toBe(false);
      return new Response(JSON.stringify([]), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const client = createApiClient('/api');
    await expect(client.scanSources('session')).resolves.toEqual([]);
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

describe('createApiClient releaseMountedISOs', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('posts to release mounted ISOs and returns the cleanup summary', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      expect(String(input)).toMatch(/\/api\/iso\/release-mounted$/);
      expect(init?.method).toBe('POST');
      return new Response(JSON.stringify({ released: 2, skippedInUse: 1, failed: 0 }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const client = createApiClient('/api');
    await expect(client.releaseMountedISOs('session')).resolves.toEqual({
      released: 2,
      skippedInUse: 1,
      failed: 0,
    });
  });
});
