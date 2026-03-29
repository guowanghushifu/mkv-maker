import type { Draft, DraftTrack, Job, ParsedBDInfo, SourceEntry } from './types';

type SubmitJobRequest = {
  source: SourceEntry;
  bdinfo: ParsedBDInfo;
  draft: Draft;
  outputFilename: string;
  outputPath: string;
};

type ListJobsResponse = {
  jobs: Job[];
};

const playlistPattern = /(\d{5}\.MPLS)/i;
const titlePattern = /disc title:\s*(.+)$/im;
const durationPattern = /length:\s*([0-9:.\s]+)$/im;
const sanitizeCharsPattern = /[<>:"/\\|?*\x00-\x1f]/g;

let localJobs: Job[] = [];
const localJobLogs = new Map<string, string>();

const fallbackSources: SourceEntry[] = [
  {
    id: 'src-1',
    name: 'Demo Disc A',
    path: '/media/discs/demo-disc-a/BDMV',
    type: 'bdmv',
    size: 98_345_308_123,
    modifiedAt: new Date().toISOString(),
  },
  {
    id: 'src-2',
    name: 'Demo Disc B',
    path: '/media/discs/demo-disc-b/BDMV',
    type: 'bdmv',
    size: 76_954_883_245,
    modifiedAt: new Date().toISOString(),
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

function normalizeCodecLabel(value: string): string {
  return value
    .replace(/[()[\]]/g, ' ')
    .replace(/\s+/g, '.')
    .replace(/[^A-Za-z0-9.+-]/g, '')
    .replace(/\.+/g, '.')
    .replace(/^\.+|\.+$/g, '');
}

function sanitizeFilename(name: string): string {
  return name
    .replace(sanitizeCharsPattern, '')
    .replace(/\s+/g, ' ')
    .replace(/\.+/g, '.')
    .trim();
}

function buildHDRLabel(hdrType: string | undefined, dvMergeEnabled: boolean | undefined): string {
  const hdr = (hdrType || '').toUpperCase();
  if (dvMergeEnabled || hdr.includes('DV')) {
    return 'HDR.DV';
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
  const defaultAudioCodec = normalizeCodecLabel(
    defaultAudio?.codecLabel || defaultAudio?.name || 'UnknownAudio'
  );

  const left = [title, resolution].filter(Boolean).join(' - ');
  const parts = [left, 'BluRay', hdr, videoCodec, defaultAudioCodec].filter(
    (part) => part.trim().length > 0
  );
  return `${sanitizeFilename(parts.join('.'))}.mkv`;
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
    codecLabel: normalizeCodecLabel(name),
    selected: true,
    default: isDefault,
  };
}

function fallbackDraft(source: SourceEntry, bdinfo: ParsedBDInfo): Draft {
  const audioNames = bdinfo.audioLabels.length > 0 ? bdinfo.audioLabels : ['English TrueHD 7.1 Atmos'];
  const subtitleNames = bdinfo.subtitleLabels.length > 0 ? bdinfo.subtitleLabels : ['English PGS'];
  const dvMergeEnabled = /dolby\s+vision|hdr\.dv/i.test(bdinfo.rawText);

  return {
    sourceId: source.id,
    playlistName: bdinfo.playlistName,
    outputDir: '/output',
    title: bdinfo.discTitle || source.name,
    dvMergeEnabled,
    video: {
      name: 'Main Video',
      codec: 'HEVC',
      resolution: '2160p',
      hdrType: dvMergeEnabled ? 'HDR.DV' : 'HDR',
    },
    audio: audioNames.map((name, index) => makeTrack('a', name, index, index === 0)),
    subtitles: subtitleNames.map((name, index) => ({
      ...makeTrack('s', name, index, index === 0),
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

function normalizeJob(partial: Partial<Job>): Job {
  return {
    id: partial.id || `job-${Date.now()}`,
    sourceName: partial.sourceName || 'Unknown Source',
    outputName: partial.outputName || 'pending.mkv',
    outputPath: partial.outputPath || '/output/pending.mkv',
    playlistName: partial.playlistName || 'unknown',
    createdAt: partial.createdAt || new Date().toISOString(),
    status: partial.status || 'queued',
    message: partial.message,
  };
}

export function createApiClient(basePath = '/api') {
  return {
    async login(password: string): Promise<{ token: string }> {
      try {
        return await requestJSON<{ token: string }>(`${basePath}/login`, {
          method: 'POST',
          body: JSON.stringify({ password }),
        });
      } catch {
        return { token: `local-${Date.now()}` };
      }
    },

    async scanSources(token?: string): Promise<SourceEntry[]> {
      try {
        return await requestJSON<SourceEntry[]>(`${basePath}/sources/scan`, { method: 'POST' }, token);
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
          `${basePath}/sources/${source.id}/resolve`,
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

    async previewFilename(draft: Draft, fallbackTitle: string, token?: string): Promise<string> {
      try {
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
                  codecLabel: track.codecLabel || normalizeCodecLabel(track.name),
                  default: track.default,
                  selected: track.selected,
                })),
            }),
          },
          token
        );
        return response.filename;
      } catch {
        return buildFilenamePreview(draft, fallbackTitle);
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
        const job: Job = {
          id: `job-${Date.now()}`,
          sourceName: payload.source.name,
          outputName: payload.outputFilename,
          outputPath: payload.outputPath,
          playlistName: payload.bdinfo.playlistName,
          createdAt: new Date().toISOString(),
          status: 'queued',
        };
        localJobs = [job, ...localJobs];
        localJobLogs.set(
          job.id,
          `[${new Date().toISOString()}] queued\nResolving playlist ${job.playlistName}\nPreparing output ${job.outputPath}`
        );
        return job;
      }
    },

    async listJobs(token?: string): Promise<Job[]> {
      try {
        const payload = await requestJSON<Job[] | ListJobsResponse>(`${basePath}/jobs`, { method: 'GET' }, token);
        const items = Array.isArray(payload) ? payload : payload.jobs;
        return items.map((job) => normalizeJob(job));
      } catch {
        return localJobs;
      }
    },

    async getJobLog(jobId: string, token?: string): Promise<string> {
      try {
        const response = await fetch(`${basePath}/jobs/${jobId}/log`, {
          method: 'GET',
          headers: token ? { Authorization: `Bearer ${token}` } : undefined,
        });
        if (!response.ok) {
          throw new Error(`Request failed with status ${response.status}`);
        }
        return await response.text();
      } catch {
        return (
          localJobLogs.get(jobId) ||
          `[${new Date().toISOString()}] Log endpoint not implemented yet.\nThis is placeholder content for job ${jobId}.`
        );
      }
    },
  };
}
