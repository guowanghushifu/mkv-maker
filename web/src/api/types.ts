export type SourceType = 'bdmv' | 'iso';

export type SourceEntry = {
  id: string;
  name: string;
  path: string;
  type: SourceType;
  size: number;
  modifiedAt: string;
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
  sourceIndex: number;
  name: string;
  language: string;
  codecLabel?: string;
  selected: boolean;
  default: boolean;
  forced?: boolean;
};

export type MakeMKVCache = {
  playlistName: string;
  titleId: number;
  audio: DraftTrack[];
  subtitles: DraftTrack[];
};

export type Draft = {
  title?: string;
  sourceId?: string;
  playlistName?: string;
  outputDir?: string;
  dvMergeEnabled?: boolean;
  segmentPaths?: string[];
  video: DraftVideo;
  audio: DraftTrack[];
  subtitles: DraftTrack[];
  makemkv?: MakeMKVCache;
};

export type JobStatus = 'running' | 'succeeded' | 'failed';

export type Job = {
  id: string;
  sourceName: string;
  outputName: string;
  outputPath?: string;
  playlistName: string;
  createdAt: string;
  status: JobStatus;
  progressPercent?: number;
  commandPreview?: string;
  message?: string;
};
