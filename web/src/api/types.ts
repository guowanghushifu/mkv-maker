export type SourceType = 'bdmv';

export type SourceEntry = {
  id: string;
  name: string;
  path: string;
  type: SourceType;
  lastScannedAt?: string;
};

export type ParsedBDInfo = {
  playlistName: string;
  discTitle?: string;
  duration?: string;
  audioLabels: string[];
  subtitleLabels: string[];
  rawText: string;
};

export type DraftVideo = {
  name: string;
  codec: string;
  resolution: string;
  hdrType?: string;
};

export type DraftTrack = {
  id: string;
  name: string;
  language: string;
  selected: boolean;
  default: boolean;
  forced?: boolean;
};

export type Draft = {
  title?: string;
  sourceId?: string;
  playlistName?: string;
  video: DraftVideo;
  audio: DraftTrack[];
  subtitles: DraftTrack[];
};

export type JobStatus = 'queued' | 'running' | 'succeeded' | 'failed';

export type Job = {
  id: string;
  sourceName: string;
  outputName: string;
  playlistName: string;
  createdAt: string;
  status: JobStatus;
  message?: string;
};

