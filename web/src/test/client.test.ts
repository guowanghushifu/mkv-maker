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
            sourceIndex: 0,
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
            sourceIndex: 0,
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

  it('posts to stop the current job', async () => {
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      expect(String(input)).toMatch(/\/api\/jobs\/current\/stop$/);
      expect(init?.method).toBe('POST');
      return new Response('', { status: 202 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const client = createApiClient('/api');
    await expect(client.stopCurrentJob('session')).resolves.toBeUndefined();
  });
});

describe('createApiClient draft track builders', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('returns draft tracks with sourceIndex and synthetic ids', async () => {
    const fetchMock = vi.fn(async () => {
      return new Response(
        JSON.stringify({
          title: 'Nightcrawler',
          outputDir: '/remux',
          dvMergeEnabled: true,
          video: { name: 'Main Video', codec: 'HEVC', resolution: '2160p', hdrType: 'DV.HDR' },
          audio: [
            {
              id: 'audio-0',
              sourceIndex: 0,
              name: 'English Atmos',
              language: 'eng',
              codecLabel: 'TrueHD.7.1.Atmos',
              selected: true,
              default: true,
            },
          ],
          subtitles: [
            {
              id: 'subtitle-0',
              sourceIndex: 1,
              name: 'Signs',
              language: 'eng',
              selected: true,
              default: false,
            },
          ],
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }
      );
    });
    vi.stubGlobal('fetch', fetchMock);

    const client = createApiClient('/api');
    const draft = await client.createDraft(
      {
        id: 'disc-1',
        name: 'Nightcrawler Disc',
        path: '/bd_input/Nightcrawler/BDMV',
        type: 'bdmv',
        size: 1,
        modifiedAt: '2026-03-29T12:00:00Z',
      },
      {
        playlistName: '00800.MPLS',
        rawText: 'PLAYLIST REPORT',
        audioLabels: [],
        subtitleLabels: [],
      },
      'session'
    );

    expect(draft.audio[0]).toMatchObject({
      id: 'audio-0',
      sourceIndex: 0,
    });
    expect(draft.subtitles[0]).toMatchObject({
      id: 'subtitle-0',
      sourceIndex: 1,
    });
  });
});

describe('createApiClient bdinfo error handling', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('surfaces plain-text parse errors returned by the backend', async () => {
    const fetchMock = vi.fn(async () => {
      return new Response('missing playlist name\n', {
        status: 400,
        headers: { 'Content-Type': 'text/plain; charset=utf-8' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const client = createApiClient('/api');
    await expect(client.parseBDInfo('bad payload', 'session')).rejects.toThrow('missing playlist name');
  });

  it('surfaces plain-text resolve errors returned by the backend', async () => {
    const fetchMock = vi.fn(async () => {
      return new Response('playlist does not exist in selected source\n', {
        status: 400,
        headers: { 'Content-Type': 'text/plain; charset=utf-8' },
      });
    });
    vi.stubGlobal('fetch', fetchMock);

    const client = createApiClient('/api');
    await expect(
      client.createDraft(
        {
          id: 'disc-1',
          name: 'Nightcrawler Disc',
          path: '/bd_input/Nightcrawler/BDMV',
          type: 'bdmv',
          size: 1,
          modifiedAt: '2026-03-29T12:00:00Z',
        },
        {
          playlistName: '00800.MPLS',
          rawText: 'PLAYLIST REPORT',
          audioLabels: [],
          subtitleLabels: [],
        },
        'session'
      )
    ).rejects.toThrow('playlist does not exist in selected source');
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
