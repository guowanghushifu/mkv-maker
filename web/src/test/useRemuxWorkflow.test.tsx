import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { localeStorageKey, tokenStorageKey } from '../i18n';
import { useRemuxWorkflow } from '../useRemuxWorkflow';
import { workflowStorageKey } from '../workflowState';

const remuxCompletionAlertMock = vi.hoisted(() => ({
  playRemuxCompletionChime: vi.fn(() => Promise.resolve()),
  prepareRemuxCompletionAlerts: vi.fn(() => Promise.resolve()),
  showRemuxCompletionNotification: vi.fn(),
}));

vi.mock('../remuxCompletionAlert', () => remuxCompletionAlertMock);

const source = {
  id: 'disc-1',
  name: 'Nightcrawler Disc',
  path: '/bd_input/Nightcrawler/BDMV',
  type: 'bdmv' as const,
  size: 1,
  modifiedAt: '2026-03-29T12:00:00Z',
};

const isoSource = {
  id: 'iso-1',
  name: 'Nightcrawler ISO',
  path: '/bd_input/Nightcrawler.iso',
  type: 'iso' as const,
  size: 1,
  modifiedAt: '2026-03-29T12:00:00Z',
};

const parsedBDInfo = {
  playlistName: '00800.MPLS',
  rawText: 'PLAYLIST REPORT',
  audioLabels: ['TrueHD'],
  subtitleLabels: ['English'],
};

const draft = {
  title: 'Nightcrawler',
  outputDir: '/remux',
  dvMergeEnabled: true,
  video: { name: 'Main Video', codec: 'HEVC', resolution: '2160p', hdrType: 'DV.HDR' },
  audio: [{ id: 'a1', name: 'English', language: 'eng', selected: true, default: true }],
  subtitles: [{ id: 's1', name: 'English', language: 'eng', selected: true, default: true }],
};

function installFetchMock({
  currentJob = null,
  currentLog = '',
  scanSources = [source],
}: {
  currentJob?: Record<string, unknown> | null;
  currentLog?: string;
  scanSources?: typeof source[];
}) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const method = init?.method || 'GET';

    if (url.endsWith('/api/jobs/current') && method === 'GET') {
      if (!currentJob) {
        return new Response('', { status: 404 });
      }
      return new Response(JSON.stringify(currentJob), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    if (url.endsWith('/api/jobs/current/log') && method === 'GET') {
      if (!currentJob) {
        return new Response('', { status: 404 });
      }
      return new Response(currentLog, { status: 200 });
    }

    if (url.endsWith('/api/sources/scan') && method === 'POST') {
      return new Response(JSON.stringify(scanSources), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    if (url.endsWith('/api/iso/release-mounted') && method === 'POST') {
      return new Response(JSON.stringify({ released: 1, skippedInUse: 0, failed: 0 }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('', { status: 500 });
  });

  vi.stubGlobal('fetch', fetchMock);
}

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
  window.localStorage.clear();
});

describe('useRemuxWorkflow', () => {
  it('hydrates workflow context from persisted state when a session exists', async () => {
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: false,
      }),
    );
    installFetchMock({});

    const { result } = renderHook(() => useRemuxWorkflow());

    await waitFor(() => {
      expect(result.current.currentStep).toBe('review');
      expect(result.current.layoutContext.source).toBe('Nightcrawler Disc');
      expect(result.current.layoutContext.playlist).toBe('00800.MPLS');
      expect(result.current.layoutContext.output).toBe('Nightcrawler - 2160p.mkv');
      expect(result.current.layoutContext.task).toBe('Ready');
    });
  });

  it('surfaces running job status in the layout context after polling current job', async () => {
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: false,
      }),
    );
    installFetchMock({
      currentJob: {
        id: 'job-123',
        sourceName: 'Nightcrawler Disc',
        outputName: 'Nightcrawler - 2160p.mkv',
        outputPath: '/remux/Nightcrawler - 2160p.mkv',
        playlistName: '00800.MPLS',
        createdAt: '2026-03-29T12:00:00Z',
        status: 'running',
      },
      currentLog: '[2026-03-29T12:00:00Z] remux started',
    });

    const { result } = renderHook(() => useRemuxWorkflow());

    await waitFor(() => {
      expect(result.current.currentJob?.status).toBe('running');
      expect(result.current.layoutContext.task).toBe('Running');
    });
  });

  it('does not alert for a historical succeeded job restored on load', async () => {
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: false,
      }),
    );
    installFetchMock({
      currentJob: {
        id: 'job-123',
        sourceName: 'Nightcrawler Disc',
        outputName: 'Nightcrawler - 2160p.mkv',
        outputPath: '/remux/Nightcrawler - 2160p.mkv',
        playlistName: '00800.MPLS',
        createdAt: '2026-03-29T12:00:00Z',
        status: 'succeeded',
      },
      currentLog: '[2026-03-29T12:10:00Z] remux finished',
    });

    const { result } = renderHook(() => useRemuxWorkflow());

    await waitFor(() => {
      expect(result.current.currentJob?.status).toBe('succeeded');
    });

    expect(remuxCompletionAlertMock.playRemuxCompletionChime).not.toHaveBeenCalled();
    expect(remuxCompletionAlertMock.showRemuxCompletionNotification).not.toHaveBeenCalled();
  });

  it('alerts once when the started remux changes from running to succeeded', async () => {
    vi.useFakeTimers();
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: false,
      }),
    );

    let currentStatus: 'running' | 'succeeded' = 'running';
    let submitted = false;
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';

      if (url.endsWith('/api/jobs') && method === 'POST') {
        submitted = true;
        return new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-03T00:00:00Z',
            status: 'running',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }

      if (url.endsWith('/api/jobs/current') && method === 'GET') {
        if (!submitted) {
          return new Response('', { status: 404 });
        }
        return new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-03T00:00:00Z',
            status: currentStatus,
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }

      if (url.endsWith('/api/jobs/current/log') && method === 'GET') {
        if (!submitted) {
          return new Response('', { status: 404 });
        }
        return new Response(
          currentStatus === 'running'
            ? '[2026-04-03T00:00:00Z] remux started'
            : '[2026-04-03T00:10:00Z] remux finished',
          { status: 200 },
        );
      }

      return new Response('', { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());
    await act(async () => {
      await Promise.resolve();
    });

    expect(result.current.currentJob).toBeNull();

    await act(async () => {
      await result.current.handleSubmitJob();
    });

    expect(remuxCompletionAlertMock.prepareRemuxCompletionAlerts).toHaveBeenCalledTimes(1);
    expect(result.current.currentJob?.status).toBe('running');

    currentStatus = 'succeeded';

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(result.current.currentJob?.status).toBe('succeeded');
    expect(remuxCompletionAlertMock.playRemuxCompletionChime).toHaveBeenCalledTimes(1);
    expect(remuxCompletionAlertMock.showRemuxCompletionNotification).toHaveBeenCalledTimes(1);
    expect(remuxCompletionAlertMock.showRemuxCompletionNotification).toHaveBeenCalledWith({
      title: 'Remux completed',
      body: 'Output ready: Nightcrawler - 2160p.mkv',
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(remuxCompletionAlertMock.playRemuxCompletionChime).toHaveBeenCalledTimes(1);
    expect(remuxCompletionAlertMock.showRemuxCompletionNotification).toHaveBeenCalledTimes(1);
  });

  it('uses the latest locale copy when the remux completes after locale toggles', async () => {
    vi.useFakeTimers();
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: false,
      }),
    );

    let currentStatus: 'running' | 'succeeded' = 'running';
    let submitted = false;
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';

      if (url.endsWith('/api/jobs') && method === 'POST') {
        submitted = true;
        return new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-03T00:00:00Z',
            status: 'running',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }

      if (url.endsWith('/api/jobs/current') && method === 'GET') {
        if (!submitted) {
          return new Response('', { status: 404 });
        }
        return new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-03T00:00:00Z',
            status: currentStatus,
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }

      if (url.endsWith('/api/jobs/current/log') && method === 'GET') {
        if (!submitted) {
          return new Response('', { status: 404 });
        }
        return new Response(
          currentStatus === 'running'
            ? '[2026-04-03T00:00:00Z] remux started'
            : '[2026-04-03T00:10:00Z] remux finished',
          { status: 200 },
        );
      }

      return new Response('', { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());
    await act(async () => {
      await Promise.resolve();
    });

    await act(async () => {
      await result.current.handleSubmitJob();
    });

    await act(async () => {
      result.current.toggleLocale();
    });

    currentStatus = 'succeeded';

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(result.current.locale).toBe('zh');
    expect(remuxCompletionAlertMock.showRemuxCompletionNotification).toHaveBeenCalledWith({
      title: 'Remux 已完成',
      body: '输出已就绪：Nightcrawler - 2160p.mkv',
    });
  });

  it('does not alert when the started remux changes from running to failed', async () => {
    vi.useFakeTimers();
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: false,
      }),
    );

    let currentStatus: 'running' | 'failed' = 'running';
    let submitted = false;
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';

      if (url.endsWith('/api/jobs') && method === 'POST') {
        submitted = true;
        return new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-03T00:00:00Z',
            status: 'running',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }

      if (url.endsWith('/api/jobs/current') && method === 'GET') {
        if (!submitted) {
          return new Response('', { status: 404 });
        }
        return new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-03T00:00:00Z',
            status: currentStatus,
            message: currentStatus === 'failed' ? 'Remux failed.' : undefined,
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }

      if (url.endsWith('/api/jobs/current/log') && method === 'GET') {
        if (!submitted) {
          return new Response('', { status: 404 });
        }
        return new Response(
          currentStatus === 'running'
            ? '[2026-04-03T00:00:00Z] remux started'
            : '[2026-04-03T00:10:00Z] remux failed',
          { status: 200 },
        );
      }

      return new Response('', { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());
    await act(async () => {
      await Promise.resolve();
    });

    expect(result.current.currentJob).toBeNull();

    await act(async () => {
      await result.current.handleSubmitJob();
    });

    expect(remuxCompletionAlertMock.prepareRemuxCompletionAlerts).toHaveBeenCalledTimes(1);

    currentStatus = 'failed';

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(result.current.currentJob?.status).toBe('failed');

    expect(remuxCompletionAlertMock.playRemuxCompletionChime).not.toHaveBeenCalled();
    expect(remuxCompletionAlertMock.showRemuxCompletionNotification).not.toHaveBeenCalled();
  });

  it('remux submission still proceeds when prepareRemuxCompletionAlerts rejects', async () => {
    remuxCompletionAlertMock.prepareRemuxCompletionAlerts.mockRejectedValueOnce(new Error('no alerts'));
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: false,
      }),
    );

    let submitted = false;
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';

      if (url.endsWith('/api/jobs') && method === 'POST') {
        submitted = true;
        return new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-03T00:00:00Z',
            status: 'running',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }

      if (url.endsWith('/api/jobs/current') && method === 'GET') {
        if (!submitted) {
          return new Response('', { status: 404 });
        }
        return new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-03T00:00:00Z',
            status: 'running',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }

      if (url.endsWith('/api/jobs/current/log') && method === 'GET') {
        if (!submitted) {
          return new Response('', { status: 404 });
        }
        return new Response('[2026-04-03T00:00:00Z] remux started', { status: 200 });
      }

      return new Response('', { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());

    await act(async () => {
      await result.current.handleSubmitJob();
    });

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalledWith(
        '/api/jobs',
        expect.objectContaining({
          method: 'POST',
        }),
      );
      expect(result.current.currentJob?.id).toBe('job-123');
      expect(result.current.currentJob?.status).toBe('running');
      expect(result.current.submitError).toBeNull();
    });
  });

  it('clears armed alert state and ignores stale success snapshots after unauthorized reset', async () => {
    vi.useFakeTimers();
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: false,
      }),
    );

    let submitted = false;
    let currentJobRequests = 0;
    let resolveStaleJob: ((response: Response) => void) | null = null;
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';

      if (url.endsWith('/api/jobs') && method === 'POST') {
        submitted = true;
        return Promise.resolve(
          new Response(
            JSON.stringify({
              id: 'job-123',
              sourceName: 'Nightcrawler Disc',
              outputName: 'Nightcrawler - 2160p.mkv',
              outputPath: '/remux/Nightcrawler - 2160p.mkv',
              playlistName: '00800.MPLS',
              createdAt: '2026-04-03T00:00:00Z',
              status: 'running',
            }),
            { status: 200, headers: { 'Content-Type': 'application/json' } },
          ),
        );
      }

      if (url.endsWith('/api/jobs/current/stop') && method === 'POST') {
        return Promise.resolve(new Response('', { status: 401 }));
      }

      if (url.endsWith('/api/jobs/current') && method === 'GET') {
        if (!submitted) {
          return Promise.resolve(new Response('', { status: 404 }));
        }
        currentJobRequests += 1;
        if (currentJobRequests === 1) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                id: 'job-123',
                sourceName: 'Nightcrawler Disc',
                outputName: 'Nightcrawler - 2160p.mkv',
                outputPath: '/remux/Nightcrawler - 2160p.mkv',
                playlistName: '00800.MPLS',
                createdAt: '2026-04-03T00:00:00Z',
                status: 'running',
              }),
              { status: 200, headers: { 'Content-Type': 'application/json' } },
            ),
          );
        }
        return new Promise((resolve) => {
          resolveStaleJob = resolve;
        });
      }

      if (url.endsWith('/api/jobs/current/log') && method === 'GET') {
        if (!submitted) {
          return Promise.resolve(new Response('', { status: 404 }));
        }
        return Promise.resolve(
          new Response('[2026-04-03T00:00:00Z] remux started', { status: 200 }),
        );
      }

      return Promise.resolve(new Response('', { status: 500 }));
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());
    await act(async () => {
      await Promise.resolve();
    });

    await act(async () => {
      await result.current.handleSubmitJob();
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(resolveStaleJob).not.toBeNull();

    await act(async () => {
      await result.current.handleStopCurrentJob();
    });

    expect(result.current.token).toBeNull();
    expect(result.current.currentStep).toBe('login');

    await act(async () => {
      resolveStaleJob?.(
        new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-03T00:00:00Z',
            status: 'succeeded',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        ),
      );
      await Promise.resolve();
    });

    expect(result.current.currentJob).toBeNull();
    expect(remuxCompletionAlertMock.playRemuxCompletionChime).not.toHaveBeenCalled();
    expect(remuxCompletionAlertMock.showRemuxCompletionNotification).not.toHaveBeenCalled();
  });

  it('clears armed alert state and invalidates stale snapshots when starting the next remux', async () => {
    vi.useFakeTimers();
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: false,
      }),
    );

    let submitted = false;
    let currentJobRequests = 0;
    let resolveStaleJob: ((response: Response) => void) | null = null;
    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';

      if (url.endsWith('/api/jobs') && method === 'POST') {
        submitted = true;
        return Promise.resolve(
          new Response(
            JSON.stringify({
              id: 'job-123',
              sourceName: 'Nightcrawler Disc',
              outputName: 'Nightcrawler - 2160p.mkv',
              outputPath: '/remux/Nightcrawler - 2160p.mkv',
              playlistName: '00800.MPLS',
              createdAt: '2026-04-03T00:00:00Z',
              status: 'running',
            }),
            { status: 200, headers: { 'Content-Type': 'application/json' } },
          ),
        );
      }

      if (url.endsWith('/api/jobs/current') && method === 'GET') {
        if (!submitted) {
          return Promise.resolve(new Response('', { status: 404 }));
        }
        currentJobRequests += 1;
        if (currentJobRequests === 1) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                id: 'job-123',
                sourceName: 'Nightcrawler Disc',
                outputName: 'Nightcrawler - 2160p.mkv',
                outputPath: '/remux/Nightcrawler - 2160p.mkv',
                playlistName: '00800.MPLS',
                createdAt: '2026-04-03T00:00:00Z',
                status: 'running',
              }),
              { status: 200, headers: { 'Content-Type': 'application/json' } },
            ),
          );
        }
        return new Promise((resolve) => {
          resolveStaleJob = resolve;
        });
      }

      if (url.endsWith('/api/jobs/current/log') && method === 'GET') {
        if (!submitted) {
          return Promise.resolve(new Response('', { status: 404 }));
        }
        return Promise.resolve(new Response('[2026-04-03T00:00:00Z] remux started', { status: 200 }));
      }

      if (url.endsWith('/api/sources/scan') && method === 'POST') {
        return Promise.resolve(
          new Response(JSON.stringify([source]), {
            status: 200,
            headers: { 'Content-Type': 'application/json' },
          }),
        );
      }

      return Promise.resolve(new Response('', { status: 500 }));
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());
    await act(async () => {
      await Promise.resolve();
    });

    await act(async () => {
      await result.current.handleSubmitJob();
    });

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(resolveStaleJob).not.toBeNull();

    await act(async () => {
      await result.current.handleStartNextRemux();
    });

    expect(result.current.step).toBe('scan');
    expect(result.current.currentJob).toBeNull();

    await act(async () => {
      resolveStaleJob?.(
        new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-03T00:00:00Z',
            status: 'succeeded',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        ),
      );
      await Promise.resolve();
    });

    expect(result.current.currentJob).toBeNull();
    expect(remuxCompletionAlertMock.playRemuxCompletionChime).not.toHaveBeenCalled();
    expect(remuxCompletionAlertMock.showRemuxCompletionNotification).not.toHaveBeenCalled();
  });

  it('does not show a stale completion notification after reset when the chime promise resolves later', async () => {
    vi.useFakeTimers();
    let resolveChime: (() => void) | null = null;
    remuxCompletionAlertMock.playRemuxCompletionChime.mockImplementationOnce(
      () =>
        new Promise<void>((resolve) => {
          resolveChime = resolve;
        }),
    );
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: false,
      }),
    );

    let currentStatus: 'running' | 'succeeded' = 'running';
    let submitted = false;
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';

      if (url.endsWith('/api/jobs') && method === 'POST') {
        submitted = true;
        return new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-03T00:00:00Z',
            status: 'running',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }

      if (url.endsWith('/api/jobs/current') && method === 'GET') {
        if (!submitted) {
          return new Response('', { status: 404 });
        }
        return new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-03T00:00:00Z',
            status: currentStatus,
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } },
        );
      }

      if (url.endsWith('/api/jobs/current/log') && method === 'GET') {
        if (!submitted) {
          return new Response('', { status: 404 });
        }
        return new Response(
          currentStatus === 'running'
            ? '[2026-04-03T00:00:00Z] remux started'
            : '[2026-04-03T00:10:00Z] remux finished',
          { status: 200 },
        );
      }

      if (url.endsWith('/api/sources/scan') && method === 'POST') {
        return new Response(JSON.stringify([source]), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        });
      }

      return new Response('', { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());
    await act(async () => {
      await Promise.resolve();
    });

    await act(async () => {
      await result.current.handleSubmitJob();
    });

    currentStatus = 'succeeded';

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(remuxCompletionAlertMock.playRemuxCompletionChime).toHaveBeenCalledTimes(1);
    expect(resolveChime).not.toBeNull();

    await act(async () => {
      await result.current.handleStartNextRemux();
    });

    expect(result.current.currentJob).toBeNull();

    await act(async () => {
      resolveChime?.();
      await Promise.resolve();
    });

    expect(remuxCompletionAlertMock.showRemuxCompletionNotification).not.toHaveBeenCalled();
  });

  it('stops the running remux and refreshes the current task snapshot', async () => {
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: false,
      }),
    );

    let stopped = false;
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';

      if (url.endsWith('/api/jobs/current/stop') && method === 'POST') {
        stopped = true;
        return new Response('', { status: 202 });
      }
      if (url.endsWith('/api/jobs/current') && method === 'GET') {
        return new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-02T00:00:00Z',
            status: stopped ? 'failed' : 'running',
            message: stopped ? 'Remux canceled.' : undefined,
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } }
        );
      }
      if (url.endsWith('/api/jobs/current/log') && method === 'GET') {
        return new Response(
          stopped ? '[2026-04-02T00:00:01Z] remux canceled' : '[2026-04-02T00:00:00Z] remux started',
          { status: 200 }
        );
      }
      if (url.endsWith('/api/sources/scan') && method === 'POST') {
        return new Response(JSON.stringify([source]), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        });
      }
      return new Response('', { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());
    await waitFor(() => expect(result.current.currentJob?.status).toBe('running'));

    await act(async () => {
      await result.current.handleStopCurrentJob();
    });

    await waitFor(() => {
      expect(result.current.currentJob?.status).toBe('failed');
      expect(result.current.currentJob?.message).toBe('Remux canceled.');
      expect(result.current.currentJobLog).toContain('canceled');
    });
  });

  it('refreshes the log again when a terminal snapshot first lands with a stale log', async () => {
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: false,
      }),
    );

    let stopped = false;
    let logRequests = 0;
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';

      if (url.endsWith('/api/jobs/current/stop') && method === 'POST') {
        stopped = true;
        return new Response('', { status: 202 });
      }
      if (url.endsWith('/api/jobs/current') && method === 'GET') {
        return new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-02T00:00:00Z',
            status: stopped ? 'failed' : 'running',
            message: stopped ? 'Remux canceled.' : undefined,
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } }
        );
      }
      if (url.endsWith('/api/jobs/current/log') && method === 'GET') {
        logRequests += 1;
        if (!stopped) {
          return new Response('[2026-04-02T00:00:00Z] remux started', { status: 200 });
        }
        return new Response(
          logRequests === 2
            ? '[2026-04-02T00:00:00Z] remux started'
            : '[2026-04-02T00:00:01Z] remux canceled',
          { status: 200 }
        );
      }
      return new Response('', { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());
    await waitFor(() => expect(result.current.currentJob?.status).toBe('running'));

    await act(async () => {
      await result.current.handleStopCurrentJob();
    });

    await waitFor(() => {
      expect(result.current.currentJob?.status).toBe('failed');
      expect(result.current.currentJobLog).toContain('remux canceled');
    });

    expect(logRequests).toBe(3);
  });

  it('surfaces the localized stop failure message when stopping the current remux fails', async () => {
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: false,
      }),
    );

    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';

      if (url.endsWith('/api/jobs/current/stop') && method === 'POST') {
        return new Response('', { status: 500 });
      }
      if (url.endsWith('/api/jobs/current') && method === 'GET') {
        return new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-02T00:00:00Z',
            status: 'running',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } }
        );
      }
      if (url.endsWith('/api/jobs/current/log') && method === 'GET') {
        return new Response('[2026-04-02T00:00:00Z] remux started', { status: 200 });
      }
      return new Response('', { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());
    await waitFor(() => expect(result.current.currentJob?.status).toBe('running'));

    await act(async () => {
      await result.current.handleStopCurrentJob();
    });

    await waitFor(() => {
      expect(result.current.submitError).toBe('Failed to stop remux job.');
    });
  });

  it('does not keep stopFailed when stop races with a job that already finished', async () => {
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: false,
      }),
    );

    let stopAttempted = false;
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';

      if (url.endsWith('/api/jobs/current/stop') && method === 'POST') {
        stopAttempted = true;
        return new Response('', { status: 404 });
      }
      if (url.endsWith('/api/jobs/current') && method === 'GET') {
        return new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-02T00:00:00Z',
            status: stopAttempted ? 'failed' : 'running',
            message: stopAttempted ? 'Remux canceled.' : undefined,
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } }
        );
      }
      if (url.endsWith('/api/jobs/current/log') && method === 'GET') {
        return new Response(
          stopAttempted ? '[2026-04-02T00:00:01Z] remux canceled' : '[2026-04-02T00:00:00Z] remux started',
          { status: 200 }
        );
      }
      return new Response('', { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());
    await waitFor(() => expect(result.current.currentJob?.status).toBe('running'));

    await act(async () => {
      await result.current.handleStopCurrentJob();
    });

    await waitFor(() => {
      expect(result.current.currentJob?.status).toBe('failed');
      expect(result.current.currentJobLog).toContain('canceled');
      expect(result.current.submitError).toBeNull();
    });
  });

  it('does not let a stale polling snapshot overwrite the canceled job after stop', async () => {
    vi.useFakeTimers();
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: false,
      }),
    );

    let currentJobRequests = 0;
    let currentLogRequests = 0;
    let resolveStaleJob: ((response: Response) => void) | null = null;

    const fetchMock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';

      if (url.endsWith('/api/jobs/current/stop') && method === 'POST') {
        return Promise.resolve(new Response('', { status: 202 }));
      }

      if (url.endsWith('/api/jobs/current') && method === 'GET') {
        currentJobRequests += 1;
        if (currentJobRequests === 1) {
          return Promise.resolve(
            new Response(
              JSON.stringify({
                id: 'job-123',
                sourceName: 'Nightcrawler Disc',
                outputName: 'Nightcrawler - 2160p.mkv',
                outputPath: '/remux/Nightcrawler - 2160p.mkv',
                playlistName: '00800.MPLS',
                createdAt: '2026-04-02T00:00:00Z',
                status: 'running',
              }),
              { status: 200, headers: { 'Content-Type': 'application/json' } }
            )
          );
        }
        if (currentJobRequests === 2) {
          return new Promise((resolve) => {
            resolveStaleJob = resolve;
          });
        }
        return Promise.resolve(
          new Response(
            JSON.stringify({
              id: 'job-123',
              sourceName: 'Nightcrawler Disc',
              outputName: 'Nightcrawler - 2160p.mkv',
              outputPath: '/remux/Nightcrawler - 2160p.mkv',
              playlistName: '00800.MPLS',
              createdAt: '2026-04-02T00:00:00Z',
              status: 'failed',
              message: 'Remux canceled.',
            }),
            { status: 200, headers: { 'Content-Type': 'application/json' } }
          )
        );
      }

      if (url.endsWith('/api/jobs/current/log') && method === 'GET') {
        currentLogRequests += 1;
        if (currentLogRequests === 1) {
          return Promise.resolve(new Response('[2026-04-02T00:00:00Z] remux started', { status: 200 }));
        }
        return Promise.resolve(new Response('[2026-04-02T00:00:01Z] remux canceled', { status: 200 }));
      }

      return Promise.resolve(new Response('', { status: 500 }));
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());
    await act(async () => {
      await Promise.resolve();
    });

    expect(result.current.currentJob?.status).toBe('running');

    await act(async () => {
      await vi.advanceTimersByTimeAsync(1500);
    });

    expect(resolveStaleJob).not.toBeNull();

    await act(async () => {
      await result.current.handleStopCurrentJob();
    });

    expect(result.current.currentJob?.status).toBe('failed');
    expect(result.current.currentJob?.message).toBe('Remux canceled.');
    expect(result.current.currentJobLog).toContain('canceled');

    await act(async () => {
      resolveStaleJob?.(
        new Response(
          JSON.stringify({
            id: 'job-123',
            sourceName: 'Nightcrawler Disc',
            outputName: 'Nightcrawler - 2160p.mkv',
            outputPath: '/remux/Nightcrawler - 2160p.mkv',
            playlistName: '00800.MPLS',
            createdAt: '2026-04-02T00:00:00Z',
            status: 'running',
          }),
          { status: 200, headers: { 'Content-Type': 'application/json' } }
        )
      );
      await Promise.resolve();
    });

    expect(result.current.currentJob?.status).toBe('failed');
    expect(result.current.currentJob?.message).toBe('Remux canceled.');
    expect(result.current.currentJobLog).toContain('canceled');
  });

  it('clears workflow editing state and rescans sources when starting the next remux', async () => {
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    window.localStorage.setItem(
      workflowStorageKey,
      JSON.stringify({
        step: 'review',
        sources: [source],
        selectedSourceId: source.id,
        bdinfoText: 'PLAYLIST REPORT',
        parsedBDInfo,
        draft,
        filenamePreview: 'Nightcrawler - 2160p.mkv',
        outputFilename: 'Nightcrawler - 2160p.mkv',
        filenameEdited: true,
      }),
    );
    installFetchMock({});

    const { result } = renderHook(() => useRemuxWorkflow());

    await act(async () => {
      await result.current.handleStartNextRemux();
    });

    await waitFor(() => {
      expect(result.current.step).toBe('scan');
      expect(result.current.sources).toEqual([source]);
      expect(result.current.selectedSourceId).toBeNull();
      expect(result.current.bdinfoText).toBe('');
      expect(result.current.parsedBDInfo).toBeNull();
      expect(result.current.draft).toBeNull();
      expect(result.current.outputFilename).toBe('');
    });
  });

  it('releases mounted ISOs from the workflow hook', async () => {
    window.localStorage.setItem(tokenStorageKey, 'session');
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';
      if (url.endsWith('/api/jobs/current') && method === 'GET') return new Response('', { status: 404 });
      if (url.endsWith('/api/jobs/current/log') && method === 'GET') return new Response('', { status: 404 });
      if (url.endsWith('/api/iso/release-mounted') && method === 'POST') {
        return new Response(JSON.stringify({ released: 1, skippedInUse: 0, failed: 0 }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        });
      }
      return new Response('', { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());

    await act(async () => {
      await result.current.handleReleaseMountedISOs();
    });

    expect(fetchMock).toHaveBeenCalledWith('/api/iso/release-mounted', expect.objectContaining({ method: 'POST' }));
    expect(result.current.releasingMountedISOs).toBe(false);
  });

  it('clears stale scan errors after successfully releasing mounted ISOs', async () => {
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem('mkv-maker-locale', 'en');
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';

      if (url.endsWith('/api/jobs/current') && method === 'GET') return new Response('', { status: 404 });
      if (url.endsWith('/api/jobs/current/log') && method === 'GET') return new Response('', { status: 404 });
      if (url.endsWith('/api/sources/scan') && method === 'POST') {
        return new Response('', { status: 500 });
      }
      if (url.endsWith('/api/iso/release-mounted') && method === 'POST') {
        return new Response(JSON.stringify({ released: 1, skippedInUse: 0, failed: 0 }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        });
      }
      return new Response('', { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());

    await act(async () => {
      await result.current.handleScan();
    });

    expect(result.current.scanError).not.toBeNull();

    await act(async () => {
      await result.current.handleReleaseMountedISOs();
    });

    expect(result.current.scanError).toBeNull();
  });

  it('surfaces a fallback scan error when releasing mounted ISOs fails', async () => {
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';

      if (url.endsWith('/api/jobs/current') && method === 'GET') return new Response('', { status: 404 });
      if (url.endsWith('/api/jobs/current/log') && method === 'GET') return new Response('', { status: 404 });
      if (url.endsWith('/api/iso/release-mounted') && method === 'POST') {
        return new Response('', { status: 500 });
      }
      return new Response('', { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());

    await act(async () => {
      await result.current.handleReleaseMountedISOs();
    });

    expect(result.current.scanError).toBe('Failed to release mounted ISOs.');
  });

  it('surfaces a partial release summary when mounted ISOs fail on a 200 response', async () => {
    window.localStorage.setItem(tokenStorageKey, 'session');
    window.localStorage.setItem(localeStorageKey, 'en');
    const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input);
      const method = init?.method || 'GET';

      if (url.endsWith('/api/jobs/current') && method === 'GET') return new Response('', { status: 404 });
      if (url.endsWith('/api/jobs/current/log') && method === 'GET') return new Response('', { status: 404 });
      if (url.endsWith('/api/iso/release-mounted') && method === 'POST') {
        return new Response(JSON.stringify({ released: 2, skippedInUse: 1, failed: 1 }), {
          status: 200,
          headers: { 'Content-Type': 'application/json' },
        });
      }
      return new Response('', { status: 500 });
    });
    vi.stubGlobal('fetch', fetchMock);

    const { result } = renderHook(() => useRemuxWorkflow());

    await act(async () => {
      await result.current.handleReleaseMountedISOs();
    });

    expect(result.current.scanError).toContain('1 failed');
    expect(result.current.scanError).toContain('1 skipped');
  });
});
