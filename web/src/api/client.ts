import type {
  Draft,
  DraftTrack,
  Job,
  ParsedBDInfo,
  SourceEntry,
} from './types';

type SubmitJobRequest = {
  source: SourceEntry;
  bdinfo: ParsedBDInfo;
  draft: Draft;
  outputFilename: string;
  outputPath: string;
};
const sanitizeCharsPattern = /[<>:"/\\|?*\x00-\x1f]/g;

export class UnauthorizedError extends Error {
  constructor() {
    super('Unauthorized');
    this.name = 'UnauthorizedError';
  }
}

type APIErrorBody = {
  message?: string;
  error?: {
    message?: string;
  } | string;
};

function normalizeCodecLabel(value: string): string {
  return value
    .replace(/[\[\]]/g, ' ')
    .replace(/_/g, '.')
    .replace(/\s+/g, '.')
    .replace(/[^A-Za-z0-9().+-]/g, '')
    .replace(/\.+/g, '.')
    .replace(/^\.+|\.+$/g, '');
}


function sanitizeFilename(name: string): string {
  return name
    .replace(/_/g, '.')
    .replace(sanitizeCharsPattern, '')
    .replace(/\s+/g, ' ')
    .replace(/\.+/g, '.')
    .trim();
}

function buildHDRLabel(hdrType: string | undefined, dvMergeEnabled: boolean | undefined): string {
  const hdr = (hdrType || '').toUpperCase();
  if (dvMergeEnabled || hdr.includes('DV')) {
    return 'DV.HDR';
  }
  if (hdr.includes('HDR')) {
    return 'HDR';
  }
  return '';
}

export function buildFilenamePreview(draft: Draft, fallbackTitle: string): string {
  const title = (draft.title || fallbackTitle || 'Untitled').trim();
  const resolution = (draft.video.resolution || '').trim();
  const videoCodec = normalizeCodecLabel(draft.video.codec || 'UnknownVideo');
  const hdr = buildHDRLabel(draft.video.hdrType, draft.dvMergeEnabled);
  const defaultAudio =
    draft.audio.find((track) => track.selected && track.default) ||
    draft.audio.find((track) => track.selected) ||
    draft.audio[0];
  const defaultAudioCodec = (defaultAudio?.codecLabel || 'UnknownAudio').trim() || 'UnknownAudio';

  const left = [title, resolution].filter(Boolean).join(' - ');
  const parts = [left, 'BluRay', hdr, videoCodec, defaultAudioCodec].filter(
    (part) => part.trim().length > 0
  );
  return `${sanitizeFilename(parts.join('.'))}.mkv`;
}

function makeTrack(idPrefix: string, name: string, index: number, isDefault: boolean): DraftTrack {
  return {
    id: `${idPrefix}${index + 1}`,
    sourceIndex: index,
    name,
    language: 'eng',
    codecLabel: normalizeCodecLabel(name),
    selected: true,
    default: isDefault,
  };
}

async function requestJSON<T>(url: string, init?: RequestInit, token?: string): Promise<T> {
  void token;
  const headers = new Headers(init?.headers);
  headers.set('Content-Type', 'application/json');

  const response = await fetch(url, {
    ...init,
    headers,
  });

  if (response.status === 401) {
    throw new UnauthorizedError();
  }
  if (!response.ok) {
    throw new Error(await readErrorMessage(response));
  }

  if (response.status === 204) {
    return undefined as T;
  }

  return (await response.json()) as T;
}

function messageFromErrorBody(body: unknown): string | null {
  if (!body || typeof body !== 'object') {
    return null;
  }

  const candidate = body as APIErrorBody;
  if (typeof candidate.message === 'string' && candidate.message.trim()) {
    return candidate.message.trim();
  }
  if (typeof candidate.error === 'string' && candidate.error.trim()) {
    return candidate.error.trim();
  }
  if (
    candidate.error &&
    typeof candidate.error === 'object' &&
    typeof candidate.error.message === 'string' &&
    candidate.error.message.trim()
  ) {
    return candidate.error.message.trim();
  }
  return null;
}

async function readErrorMessage(response: Response): Promise<string> {
  const raw = (await response.text()).trim();
  if (!raw) {
    return `Request failed with status ${response.status}`;
  }

  try {
    const parsed = JSON.parse(raw) as unknown;
    const message = messageFromErrorBody(parsed);
    if (message) {
      return message;
    }
  } catch {
    // Fall through to the plain-text response body.
  }

  return raw;
}

function normalizeJob(partial: Partial<Job>): Job {
  return {
    id: partial.id || `job-${Date.now()}`,
    sourceName: partial.sourceName || 'Unknown Source',
    outputName: partial.outputName || 'pending.mkv',
    outputPath: partial.outputPath || '/output/pending.mkv',
    playlistName: partial.playlistName || 'unknown',
    createdAt: partial.createdAt || new Date().toISOString(),
    status: partial.status || 'running',
    progressPercent: partial.progressPercent,
    commandPreview: partial.commandPreview,
    message: partial.message,
  };
}

export function createApiClient(basePath = '/api') {
  return {
    async login(password: string): Promise<{ token: string }> {
      await requestJSON<void>(`${basePath}/login`, {
        method: 'POST',
        body: JSON.stringify({ password }),
      });
      return { token: 'session' };
    },

    async scanSources(token?: string): Promise<SourceEntry[]> {
      return await requestJSON<SourceEntry[]>(`${basePath}/sources/scan`, { method: 'POST' }, token);
    },

    async parseBDInfo(rawText: string, token?: string): Promise<ParsedBDInfo> {
      return await requestJSON<ParsedBDInfo>(
        `${basePath}/bdinfo/parse`,
        {
          method: 'POST',
          body: JSON.stringify({ rawText }),
        },
        token
      );
    },

    async createDraft(source: SourceEntry, bdinfo: ParsedBDInfo, token?: string): Promise<Draft> {
      return await requestJSON<Draft>(
        `${basePath}/sources/${source.id}/resolve`,
        {
          method: 'POST',
          body: JSON.stringify({ sourceId: source.id, bdinfo }),
        },
        token
      );
    },

    async previewFilename(draft: Draft, fallbackTitle: string, token?: string): Promise<string> {
      const response = await requestJSON<{ filename: string }>(
        `${basePath}/drafts/preview-filename`,
        {
          method: 'POST',
          body: JSON.stringify({
            title: draft.title || fallbackTitle,
            outputPath: draft.outputDir || '/output',
            enableDV: Boolean(draft.dvMergeEnabled),
            video: {
              name: draft.video.name,
              resolution: draft.video.resolution,
              codec: draft.video.codec,
              hdrType: draft.video.hdrType || '',
            },
            audio: draft.audio
              .filter((track) => track.selected)
              .map((track) => ({
                id: track.id,
                name: track.name,
                language: track.language,
                codecLabel: track.codecLabel || '',
                default: track.default,
                selected: track.selected,
              })),
          }),
        },
        token
      );
      return response.filename;
    },

    async submitJob(payload: SubmitJobRequest, token?: string): Promise<Job> {
      return await requestJSON<Job>(
        `${basePath}/jobs`,
        {
          method: 'POST',
          body: JSON.stringify(payload),
        },
        token
      );
    },

    async stopCurrentJob(token?: string): Promise<void> {
      void token;
      const response = await fetch(`${basePath}/jobs/current/stop`, {
        method: 'POST',
      });
      if (response.status === 401) {
        throw new UnauthorizedError();
      }
      if (!response.ok) {
        throw new Error(await readErrorMessage(response));
      }
    },

    async currentJob(token?: string): Promise<Job | null> {
      void token;
      const response = await fetch(`${basePath}/jobs/current`, {
        method: 'GET',
      });
      if (response.status === 401) {
        throw new UnauthorizedError();
      }
      if (response.status === 404) {
        return null;
      }
      if (!response.ok) {
        throw new Error(await readErrorMessage(response));
      }
      return normalizeJob((await response.json()) as Partial<Job>);
    },

    async currentJobLog(token?: string): Promise<string> {
      void token;
      const response = await fetch(`${basePath}/jobs/current/log`, {
        method: 'GET',
      });
      if (response.status === 401) {
        throw new UnauthorizedError();
      }
      if (response.status === 404) {
        return '';
      }
      if (!response.ok) {
        throw new Error(await readErrorMessage(response));
      }
      return await response.text();
    },
  };
}
