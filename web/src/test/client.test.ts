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
            id: 'A1',
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

  it.each([
    {
      name: 'normalizes dts hd ma alias separators',
      title: 'Alien_(1979)',
      codecLabel: 'DTS_HD.MA.5.1',
      expected: 'Alien.(1979) - 2160p.BluRay.HDR.HEVC.DTS-HD.MA.5.1.mkv',
    },
    {
      name: 'strips dolby digital side annotation',
      title: 'Nightcrawler',
      codecLabel: 'DD.5.1(side)',
      expected: 'Nightcrawler - 2160p.BluRay.HDR.HEVC.DD.5.1.mkv',
    },
    {
      name: 'converts lpcm stereo to channel layout',
      title: 'Nightcrawler',
      codecLabel: 'LPCM stereo',
      expected: 'Nightcrawler - 2160p.BluRay.HDR.HEVC.LPCM.2.0.mkv',
    },
    {
      name: 'converts aac mono to channel layout',
      title: 'Nightcrawler',
      codecLabel: 'AAC mono',
      expected: 'Nightcrawler - 2160p.BluRay.HDR.HEVC.AAC.1.0.mkv',
    },
    {
      name: 'preserves original label when codec base is not recognized',
      title: 'Nightcrawler',
      codecLabel: 'stereo',
      expected: 'Nightcrawler - 2160p.BluRay.HDR.HEVC.stereo.mkv',
    },
  ])('$name', ({ title, codecLabel, expected }) => {
    const filename = buildFilenamePreview(
      {
        title,
        video: {
          name: 'Main Video',
          codec: 'HEVC',
          resolution: '2160p',
          hdrType: 'HDR',
        },
        audio: [
          {
            id: 'A1',
            sourceIndex: 0,
            name: 'English',
            language: 'eng',
            codecLabel,
            selected: true,
            default: true,
          },
        ],
        subtitles: [],
      },
      'Nightcrawler'
    );

    expect(filename).toBe(expected);
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

  it('preserves sourceIndex and makemkv cache fields from resolve responses', async () => {
    const fetchMock = vi.fn(async () => {
      return new Response(
        JSON.stringify({
          sourceId: 'disc-1',
          playlistName: '00800.MPLS',
          outputDir: '/remux',
          title: 'Nightcrawler',
          dvMergeEnabled: true,
          segmentPaths: ['/bd_input/Nightcrawler/BDMV/STREAM/00001.m2ts'],
          video: {
            name: 'Main Video',
            codec: 'HEVC',
            resolution: '2160p',
            hdrType: 'DV.HDR',
          },
          audio: [
            {
              id: 'A1',
              sourceIndex: 7,
              name: 'English Atmos',
              language: 'eng',
              codecLabel: 'TrueHD.7.1',
              selected: true,
              default: true,
            },
          ],
          subtitles: [
            {
              id: 'S1',
              sourceIndex: 12,
              name: 'English PGS',
              language: 'eng',
              selected: true,
              default: false,
              forced: true,
            },
          ],
          makemkv: {
            playlistName: '00800.MPLS',
            titleId: 0,
            audio: [
              {
                id: 'A1',
                sourceIndex: 7,
                name: 'English Atmos',
                language: 'eng',
                codecLabel: 'TrueHD.7.1',
                selected: true,
                default: true,
              },
            ],
            subtitles: [
              {
                id: 'S1',
                sourceIndex: 12,
                name: 'English PGS',
                language: 'eng',
                selected: true,
                default: false,
                forced: true,
              },
            ],
          },
        }),
        {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        }
      );
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
    ).resolves.toMatchObject({
      audio: [{ id: 'A1', sourceIndex: 7 }],
      subtitles: [{ id: 'S1', sourceIndex: 12, forced: true }],
      makemkv: {
        playlistName: '00800.MPLS',
        titleId: 0,
        audio: [{ id: 'A1', sourceIndex: 7 }],
        subtitles: [{ id: 'S1', sourceIndex: 12, forced: true }],
      },
    });
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
