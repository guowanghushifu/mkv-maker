import type { Draft, DraftTrack, Job, ParsedBDInfo, SourceEntry } from './types';

type SubmitJobRequest = {
  source: SourceEntry;
  bdinfo: ParsedBDInfo;
  draft: Draft;
};

const playlistPattern = /(\d{5}\.MPLS)/i;
const titlePattern = /disc title:\s*(.+)$/im;
const durationPattern = /length:\s*([0-9:.\s]+)$/im;

let localJobs: Job[] = [];

const fallbackSources: SourceEntry[] = [
  {
    id: 'src-1',
    name: 'Demo Disc A',
    path: '/media/discs/demo-disc-a/BDMV',
    type: 'bdmv',
    lastScannedAt: new Date().toISOString(),
  },
  {
    id: 'src-2',
    name: 'Demo Disc B',
    path: '/media/discs/demo-disc-b/BDMV',
    type: 'bdmv',
    lastScannedAt: new Date().toISOString(),
  },
];

function extractTrackLabels(text: string, prefix: 'audio' | 'subtitle'): string[] {
  return text
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.toLowerCase().startsWith(prefix))
    .map((line) => line.replace(/^[^:]+:\s*/, '').trim())
    .filter(Boolean);
}

function fallbackParseBDInfo(rawText: string): ParsedBDInfo {
  const playlistMatch = rawText.match(playlistPattern);
  if (!playlistMatch) {
    throw new Error('Could not find a playlist in BDInfo text (expected something like 00800.MPLS).');
  }

  const discTitleMatch = rawText.match(titlePattern);
  const durationMatch = rawText.match(durationPattern);
  const audioLabels = extractTrackLabels(rawText, 'audio');
  const subtitleLabels = extractTrackLabels(rawText, 'subtitle');

  return {
    playlistName: playlistMatch[1].toUpperCase(),
    discTitle: discTitleMatch?.[1]?.trim() || undefined,
    duration: durationMatch?.[1]?.trim() || undefined,
    audioLabels,
    subtitleLabels,
    rawText,
  };
}

function makeTrack(idPrefix: string, name: string, index: number, isDefault: boolean): DraftTrack {
  return {
    id: `${idPrefix}-${index + 1}`,
    name,
    language: 'eng',
    selected: true,
    default: isDefault,
  };
}

function fallbackDraft(source: SourceEntry, bdinfo: ParsedBDInfo): Draft {
  const audioNames = bdinfo.audioLabels.length > 0 ? bdinfo.audioLabels : ['English DTS-HD MA 5.1'];
  const subtitleNames = bdinfo.subtitleLabels.length > 0 ? bdinfo.subtitleLabels : ['English PGS'];

  return {
    sourceId: source.id,
    playlistName: bdinfo.playlistName,
    title: bdinfo.discTitle || source.name,
    video: {
      name: 'Main Video',
      codec: 'HEVC',
      resolution: '2160p',
      hdrType: 'HDR.DV',
    },
    audio: audioNames.map((name, index) => makeTrack('a', name, index, index === 0)),
    subtitles: subtitleNames.map((name, index) => ({
      ...makeTrack('s', name, index, false),
      forced: false,
    })),
  };
}

async function requestJSON<T>(url: string, init?: RequestInit, token?: string): Promise<T> {
  const headers = new Headers(init?.headers);
  headers.set('Content-Type', 'application/json');
  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }

  const response = await fetch(url, {
    ...init,
    headers,
  });

  if (!response.ok) {
    throw new Error(`Request failed with status ${response.status}`);
  }

  return (await response.json()) as T;
}

export function createApiClient(basePath = '/api') {
  return {
    async login(password: string): Promise<{ token: string }> {
      try {
        return await requestJSON<{ token: string }>(`${basePath}/auth/login`, {
          method: 'POST',
          body: JSON.stringify({ password }),
        });
      } catch {
        return { token: `local-${Date.now()}` };
      }
    },

    async scanSources(token?: string): Promise<SourceEntry[]> {
      try {
        return await requestJSON<SourceEntry[]>(`${basePath}/sources`, { method: 'GET' }, token);
      } catch {
        return fallbackSources;
      }
    },

    async parseBDInfo(rawText: string, token?: string): Promise<ParsedBDInfo> {
      try {
        return await requestJSON<ParsedBDInfo>(
          `${basePath}/bdinfo/parse`,
          {
            method: 'POST',
            body: JSON.stringify({ rawText }),
          },
          token
        );
      } catch {
        return fallbackParseBDInfo(rawText);
      }
    },

    async createDraft(source: SourceEntry, bdinfo: ParsedBDInfo, token?: string): Promise<Draft> {
      try {
        return await requestJSON<Draft>(
          `${basePath}/draft/resolve`,
          {
            method: 'POST',
            body: JSON.stringify({ sourceId: source.id, bdinfo }),
          },
          token
        );
      } catch {
        return fallbackDraft(source, bdinfo);
      }
    },

    async submitJob(payload: SubmitJobRequest, token?: string): Promise<Job> {
      try {
        return await requestJSON<Job>(
          `${basePath}/jobs`,
          {
            method: 'POST',
            body: JSON.stringify(payload),
          },
          token
        );
      } catch {
        const outputName = `${(payload.draft.title || payload.source.name).replace(/\s+/g, '.')}.mkv`;
        const job: Job = {
          id: `job-${Date.now()}`,
          sourceName: payload.source.name,
          outputName,
          playlistName: payload.bdinfo.playlistName,
          createdAt: new Date().toISOString(),
          status: 'queued',
        };
        localJobs = [job, ...localJobs];
        return job;
      }
    },

    async listJobs(token?: string): Promise<Job[]> {
      try {
        return await requestJSON<Job[]>(`${basePath}/jobs`, { method: 'GET' }, token);
      } catch {
        return localJobs;
      }
    },
  };
}

