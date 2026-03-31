import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import App from '../App';
import { localeStorageKey, tokenStorageKey } from '../i18n';
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

type BackendState = {
  currentJob: Record<string, unknown> | null;
  currentLog: string;
  submitStatus?: number;
  submitMessage?: string;
  submittedJob?: Record<string, unknown>;
  submittedLog?: string;
  submitDelay?: Promise<void>;
};

function installFetchMock(state: BackendState) {
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const method = init?.method || 'GET';

    if (url.endsWith('/api/login') && method === 'POST') {
      return new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }
    if (url.endsWith('/api/sources/scan') && method === 'POST') {
      return new Response(JSON.stringify([source]), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }
    if (url.endsWith('/api/bdinfo/parse') && method === 'POST') {
      return new Response(JSON.stringify(parsedBDInfo), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }
    if (url.endsWith('/api/sources/disc-1/resolve') && method === 'POST') {
      return new Response(JSON.stringify(draft), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }
    if (url.endsWith('/api/drafts/preview-filename') && method === 'POST') {
      return new Response(JSON.stringify({ filename: 'Nightcrawler - 2160p.mkv' }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }
    if (url.endsWith('/api/jobs/current') && method === 'GET') {
      if (!state.currentJob) {
        return new Response('', { status: 404 });
      }
      return new Response(JSON.stringify(state.currentJob), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }
    if (url.endsWith('/api/jobs/current/log') && method === 'GET') {
      if (!state.currentJob) {
        return new Response('', { status: 404 });
      }
      return new Response(state.currentLog, { status: 200 });
    }
    if (url.endsWith('/api/jobs') && method === 'POST') {
      if (state.submitDelay) {
        await state.submitDelay;
      }
      if (state.submitStatus && state.submitStatus >= 400) {
        return new Response(state.submitMessage || '', { status: state.submitStatus });
      }
      if (state.submittedJob) {
        state.currentJob = state.submittedJob;
      }
      if (typeof state.submittedLog === 'string') {
        state.currentLog = state.submittedLog;
      }
      return new Response(JSON.stringify(state.currentJob), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }
    return new Response('', { status: 500 });
  });

  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

function installUnauthorizedFetchMock() {
  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = String(input);
    const method = init?.method || 'GET';

    if (url.endsWith('/api/login') && method === 'POST') {
      return new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      });
    }

    return new Response('', { status: 401 });
  });

  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

async function goToReviewStep() {
  fireEvent.click(screen.getByRole('button', { name: /中文 \/ EN/i }));
  await screen.findByRole('heading', { name: /Login/i });

  fireEvent.change(screen.getByLabelText(/password/i), { target: { value: 'secret' } });
  fireEvent.click(screen.getByRole('button', { name: /continue/i }));
  await screen.findByRole('heading', { name: /scan sources/i });

  fireEvent.click(screen.getByRole('button', { name: /scan sources/i }));
  await screen.findByLabelText(/select nightcrawler disc/i);
  fireEvent.click(screen.getByRole('button', { name: /continue to bdinfo/i }));

  await screen.findByRole('heading', { name: /required bdinfo/i });
  fireEvent.change(screen.getByPlaceholderText(/paste full bdinfo text here/i), {
    target: { value: 'PLAYLIST REPORT' },
  });
  fireEvent.click(screen.getByRole('button', { name: /parse bdinfo and continue/i }));

  await screen.findByRole('heading', { name: /track editor/i });
  fireEvent.click(screen.getByRole('button', { name: /continue to review/i }));
  await screen.findByRole('heading', { name: /^review$/i });
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
  window.localStorage.clear();
});

describe('App', () => {
  it('renders the application shell title', () => {
    installFetchMock({ currentJob: null, currentLog: '' });
    render(<App />);
    expect(screen.getByRole('heading', { name: /MKV Remux Tool/i })).toBeInTheDocument();
    expect(screen.queryByText(/Media Mastering Console/i)).not.toBeInTheDocument();
    expect(screen.getByText('任务概览')).toBeInTheDocument();
    expect(screen.queryByText('Jobs')).not.toBeInTheDocument();
  });

  it('defaults to Chinese and persists the selected locale', async () => {
    installFetchMock({ currentJob: null, currentLog: '' });
    render(<App />);

    expect(screen.getByRole('heading', { name: /登录/i })).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /中文 \/ EN/i }));

    expect(await screen.findByRole('heading', { name: /Login/i })).toBeInTheDocument();
    expect(window.localStorage.getItem(localeStorageKey)).toBe('en');
  });

  it('restores the saved locale from localStorage on load', () => {
    window.localStorage.setItem(localeStorageKey, 'en');
    installFetchMock({ currentJob: null, currentLog: '' });
    render(<App />);

    expect(screen.getByRole('heading', { name: /Login/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /continue/i })).toBeInTheDocument();
  });

  it('keeps the login session after a page refresh', async () => {
    installFetchMock({ currentJob: null, currentLog: '' });
    const view = render(<App />);

    fireEvent.change(screen.getByLabelText(/密码/i), { target: { value: 'secret' } });
    fireEvent.click(screen.getByRole('button', { name: /继续/i }));
    await screen.findByRole('heading', { name: /扫描片源/i });
    expect(window.localStorage.getItem(tokenStorageKey)).toBe('session');

    view.unmount();
    render(<App />);

    expect(await screen.findByRole('heading', { name: /扫描片源/i })).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: /登录/i })).not.toBeInTheDocument();
  });

  it('stays on the review step after a refresh when a remux is already running', async () => {
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
    const view = render(<App />);

    await goToReviewStep();
    await screen.findByText(/current remux/i);

    view.unmount();
    render(<App />);

    expect(await screen.findByRole('heading', { name: /^review$/i })).toBeInTheDocument();
    expect(await screen.findByText(/current remux/i)).toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: /scan sources/i })).not.toBeInTheDocument();
  });

  it('shows submit failure message on review when start remux request fails', async () => {
    installFetchMock({ currentJob: null, currentLog: '', submitStatus: 409 });
    render(<App />);

    await goToReviewStep();
    fireEvent.click(screen.getByRole('button', { name: /start remux/i }));

    expect(await screen.findByText(/request failed with status 409/i)).toBeInTheDocument();
  });

  it('logs out and clears local state when a protected request returns 401', async () => {
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
      })
    );
    installUnauthorizedFetchMock();
    render(<App />);

    expect(await screen.findByRole('heading', { name: /登录/i })).toBeInTheDocument();
    expect(window.localStorage.getItem(tokenStorageKey)).toBeNull();
    expect(window.localStorage.getItem(workflowStorageKey)).toBeNull();
  });

  it('clears the old log immediately when starting a new remux', async () => {
    let releaseSubmit!: () => void;
    const submitDelay = new Promise<void>((resolve) => {
      releaseSubmit = resolve;
    });

    installFetchMock({
      currentJob: {
        id: 'job-old',
        sourceName: 'Nightcrawler Disc',
        outputName: 'Nightcrawler - 2160p.mkv',
        outputPath: '/remux/Nightcrawler - 2160p.mkv',
        playlistName: '00800.MPLS',
        createdAt: '2026-03-29T12:00:00Z',
        status: 'succeeded',
      },
      currentLog: '[2026-03-29T12:00:01Z] old log line',
      submittedJob: {
        id: 'job-new',
        sourceName: 'Nightcrawler Disc',
        outputName: 'Nightcrawler - 2160p.mkv',
        outputPath: '/remux/Nightcrawler - 2160p.mkv',
        playlistName: '00800.MPLS',
        createdAt: '2026-03-30T12:00:00Z',
        status: 'running',
      },
      submittedLog: '',
      submitDelay,
    });
    render(<App />);

    await goToReviewStep();
    expect(screen.getByText(/old log line/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', { name: /start remux/i }));

    await waitFor(() => {
      expect(screen.queryByText(/old log line/i)).not.toBeInTheDocument();
      expect(screen.queryByText(/current remux/i)).not.toBeInTheDocument();
    });

    releaseSubmit();
  });

  it('hydrates current job log immediately after submit for terminal tasks', async () => {
    const terminalJob = {
      id: 'job-999',
      sourceName: 'Nightcrawler Disc',
      outputName: 'Nightcrawler - 2160p.mkv',
      outputPath: '/remux/Nightcrawler - 2160p.mkv',
      playlistName: '00800.MPLS',
      createdAt: '2026-03-29T12:00:00Z',
      status: 'succeeded',
    };
    installFetchMock({
      currentJob: null,
      currentLog: '[2026-03-29T12:00:01Z] remux completed',
      submittedJob: terminalJob,
      submitStatus: 200,
      submitMessage: '',
    });
    render(<App />);

    await goToReviewStep();
    fireEvent.click(screen.getByRole('button', { name: /start remux/i }));

    await screen.findByText(/current remux/i);
    expect((await screen.findAllByText(/succeeded/i)).length).toBeGreaterThan(0);
    expect(await screen.findByText(/remux completed/i)).toBeInTheDocument();
    expect(screen.queryByText(/waiting for log output/i)).not.toBeInTheDocument();
  });

  it('hydrates terminal tasks with command preview and 100 percent progress', async () => {
    const terminalJob = {
      id: 'job-999',
      sourceName: 'Nightcrawler Disc',
      outputName: 'Nightcrawler - 2160p.mkv',
      outputPath: '/remux/Nightcrawler - 2160p.mkv',
      playlistName: '00003.MPLS',
      createdAt: '2026-03-30T00:00:00Z',
      status: 'succeeded',
      progressPercent: 100,
      commandPreview: 'mkvmerge\n  --output\n  /remux/Nightcrawler - 2160p.mkv',
    };
    installFetchMock({
      currentJob: null,
      currentLog: '[2026-03-30T00:00:01Z] Progress: 100%\n[2026-03-30T00:00:02Z] completed',
      submittedJob: terminalJob,
      submitStatus: 200,
      submitMessage: '',
    });
    render(<App />);

    await goToReviewStep();
    fireEvent.click(screen.getByRole('button', { name: /start remux/i }));

    await screen.findByText(/current remux/i);
    expect(await screen.findByText('100%')).toBeInTheDocument();
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '100');
    expect(screen.getByText(/mkvmerge/i)).toBeInTheDocument();
    expect(screen.getByText(/--output/i)).toBeInTheDocument();
  });

  it('disables start remux while a current task is already running', async () => {
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
    render(<App />);

    await goToReviewStep();
    await waitFor(() => {
      expect(screen.getByRole('button', { name: /start remux/i })).toBeDisabled();
      expect(screen.getByRole('button', { name: /start next remux/i })).toBeDisabled();
    });
  });

  it('hydrates running current task snapshot with command preview and progress', async () => {
    installFetchMock({
      currentJob: {
        id: 'job-123',
        sourceName: 'Nightcrawler Disc',
        outputName: 'Nightcrawler - 2160p.mkv',
        outputPath: '/remux/Nightcrawler - 2160p.mkv',
        playlistName: '00800.MPLS',
        createdAt: '2026-03-29T12:00:00Z',
        status: 'running',
        progressPercent: 7,
        commandPreview: 'mkvmerge\n  --output\n  /remux/Nightcrawler - 2160p.mkv',
      },
      currentLog: '[2026-03-29T12:00:00Z] remux started',
    });
    render(<App />);

    await goToReviewStep();

    await screen.findByText(/current remux/i);
    expect(await screen.findByText('7%')).toBeInTheDocument();
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '7');
    expect(screen.getByText(/mkvmerge/i)).toBeInTheDocument();
    expect(screen.getByText(/--output/i)).toBeInTheDocument();
  });

  it('can jump back to scan for the next remux and clears prior workflow state', async () => {
    installFetchMock({
      currentJob: {
        id: 'job-999',
        sourceName: 'Nightcrawler Disc',
        outputName: 'Nightcrawler - 2160p.mkv',
        outputPath: '/remux/Nightcrawler - 2160p.mkv',
        playlistName: '00800.MPLS',
        createdAt: '2026-03-29T12:00:00Z',
        status: 'succeeded',
      },
      currentLog: '[2026-03-29T12:30:00Z] remux completed',
    });
    render(<App />);

    await goToReviewStep();
    fireEvent.click(screen.getByRole('button', { name: /start next remux/i }));

    await screen.findByRole('heading', { name: /scan sources/i });
    const sourceRadio = await screen.findByLabelText(/select nightcrawler disc/i);
    expect(screen.queryByRole('heading', { name: /^review$/i })).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: /continue to bdinfo/i })).toBeDisabled();
    expect(sourceRadio).not.toBeChecked();
  });

  it('renders workflow context cards on the review step', async () => {
    installFetchMock({ currentJob: null, currentLog: '' });
    render(<App />);

    await goToReviewStep();

    expect(screen.getByText(/current session/i)).toBeInTheDocument();
    const context = screen.getByText(/current session/i).closest('.workflow-context');
    expect(context).not.toBeNull();
    expect(screen.getAllByText(/nightcrawler disc/i).some((node) => node.closest('.context-card'))).toBe(true);
    expect(screen.getAllByText(/00800\.MPLS/i).some((node) => node.closest('.context-card'))).toBe(true);
    expect(screen.getAllByText(/nightcrawler - 2160p\.mkv/i).some((node) => node.closest('.context-card'))).toBe(true);
  });
});
