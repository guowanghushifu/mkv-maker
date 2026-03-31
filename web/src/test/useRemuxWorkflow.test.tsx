import { act, renderHook, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { localeStorageKey, tokenStorageKey } from '../i18n';
import { useRemuxWorkflow } from '../useRemuxWorkflow';
import { workflowStorageKey } from '../workflowState';

const source = {
  id: 'disc-1',
  name: 'Nightcrawler Disc',
  path: '/bd_input/Nightcrawler/BDMV',
  type: 'bdmv' as const,
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

    return new Response('', { status: 500 });
  });

  vi.stubGlobal('fetch', fetchMock);
}

afterEach(() => {
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
});
