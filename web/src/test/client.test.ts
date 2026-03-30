import { describe, expect, it } from 'vitest';
import { buildFilenamePreview } from '../api/client';

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
          hdrType: 'HDR.DV',
        },
        audio: [
          {
            id: 'a1',
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

    expect(filename).toBe('Nightcrawler - 2160p.BluRay.HDR.DV.HEVC.UnknownAudio.mkv');
  });
});
