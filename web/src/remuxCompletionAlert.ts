export type RemuxCompletionNotification = {
  title: string;
  body: string;
};

type AudioContextConstructor = new () => AudioContext;
type ChimeTone = {
  frequency: number;
  startOffsetSeconds: number;
  fadeOutSeconds: number;
  stopSeconds: number;
};

const REMUX_CHIME_ATTACK_SECONDS = 0.02;
const REMUX_CHIME_PEAK_GAIN = 0.12;
const REMUX_CHIME_REPEAT_COUNT = 5;
const REMUX_CHIME_REPEAT_INTERVAL_SECONDS = 0.55;
const REMUX_CHIME_TONES: readonly ChimeTone[] = [
  {
    frequency: 880,
    startOffsetSeconds: 0,
    fadeOutSeconds: 0.18,
    stopSeconds: 0.22,
  },
  {
    frequency: 1174.66,
    startOffsetSeconds: 0.08,
    fadeOutSeconds: 0.26,
    stopSeconds: 0.3,
  },
];

let sharedAudioContext: AudioContext | null = null;
let sharedAudioContextConstructor: AudioContextConstructor | null = null;

function getAudioContextConstructor(): AudioContextConstructor | null {
  const globalAudio = globalThis as typeof globalThis & {
    AudioContext?: AudioContextConstructor;
    webkitAudioContext?: AudioContextConstructor;
  };

  return globalAudio.AudioContext ?? globalAudio.webkitAudioContext ?? null;
}

function getAudioContext(): AudioContext | null {
  const AudioContextCtor = getAudioContextConstructor();

  if (!AudioContextCtor) {
    return null;
  }

  if (sharedAudioContext && sharedAudioContextConstructor === AudioContextCtor) {
    return sharedAudioContext;
  }

  try {
    sharedAudioContext = new AudioContextCtor();
    sharedAudioContextConstructor = AudioContextCtor;
    return sharedAudioContext;
  } catch {
    return null;
  }
}

async function resumeAudioContextIfSuspended(audioContext: AudioContext): Promise<void> {
  if (audioContext.state !== 'suspended') {
    return;
  }

  await audioContext.resume();
}

function scheduleChimeTone(
  audioContext: AudioContext,
  startTime: number,
  tone: ChimeTone,
): void {
  const oscillator = audioContext.createOscillator();
  const gain = audioContext.createGain();
  const toneStartTime = startTime + tone.startOffsetSeconds;

  oscillator.type = 'sine';
  oscillator.frequency.value = tone.frequency;
  oscillator.connect(gain);
  gain.connect(audioContext.destination);

  gain.gain.setValueAtTime(0, toneStartTime);
  gain.gain.linearRampToValueAtTime(REMUX_CHIME_PEAK_GAIN, toneStartTime + REMUX_CHIME_ATTACK_SECONDS);
  gain.gain.linearRampToValueAtTime(0, startTime + tone.fadeOutSeconds);

  oscillator.start(toneStartTime);
  oscillator.stop(startTime + tone.stopSeconds);
}

export async function prepareRemuxCompletionAlerts(): Promise<void> {
  let audioWarmupPromise: Promise<void> | null = null;
  try {
    const audioContext = getAudioContext();
    if (audioContext) {
      audioWarmupPromise = resumeAudioContextIfSuspended(audioContext).catch(() => undefined);
    }
  } catch {
    // Audio warm-up is best-effort only.
  }

  try {
    const NotificationApi = globalThis.Notification;
    if (!NotificationApi || NotificationApi.permission !== 'default') {
      return;
    }

    if (typeof NotificationApi.requestPermission !== 'function') {
      return;
    }

    await Promise.resolve(NotificationApi.requestPermission());
  } catch {
    // Keep remux submission moving even if browser alert setup fails.
  }

  if (audioWarmupPromise) {
    await audioWarmupPromise;
  }
}

export async function playRemuxCompletionChime(): Promise<void> {
  try {
    const audioContext = getAudioContext();
    if (!audioContext) {
      return;
    }

    await resumeAudioContextIfSuspended(audioContext);

    const startTime = audioContext.currentTime;
    for (let iteration = 0; iteration < REMUX_CHIME_REPEAT_COUNT; iteration += 1) {
      const repetitionStartTime = startTime + iteration * REMUX_CHIME_REPEAT_INTERVAL_SECONDS;
      for (const tone of REMUX_CHIME_TONES) {
        scheduleChimeTone(audioContext, repetitionStartTime, tone);
      }
    }
  } catch {
    // Playback should never block the remux flow.
  }
}

export function showRemuxCompletionNotification(notification: RemuxCompletionNotification): void {
  const NotificationApi = globalThis.Notification;
  if (!NotificationApi || NotificationApi.permission !== 'granted') {
    return;
  }

  try {
    const browserNotification = new NotificationApi(notification.title, {
      body: notification.body,
      tag: 'mkv-maker-remux-complete',
    });

    browserNotification.onclick = () => {
      try {
        if (typeof window !== 'undefined' && typeof window.focus === 'function') {
          window.focus();
        }
      } finally {
        browserNotification.close();
      }
    };
  } catch {
    // Notification creation must not interfere with completion handling.
  }
}
