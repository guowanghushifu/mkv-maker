export type RemuxCompletionNotification = {
  title: string;
  body: string;
};

type AudioContextConstructor = new () => AudioContext;

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

export async function prepareRemuxCompletionAlerts(): Promise<void> {
  let audioWarmupPromise: Promise<void> | null = null;
  try {
    const audioContext = getAudioContext();
    if (audioContext) {
      audioWarmupPromise = resumeAudioContextIfSuspended(audioContext);
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
    try {
      await audioWarmupPromise;
    } catch {
      // Audio warm-up is best-effort only.
    }
  }
}

export async function playRemuxCompletionChime(): Promise<void> {
  try {
    const audioContext = getAudioContext();
    if (!audioContext) {
      return;
    }

    await resumeAudioContextIfSuspended(audioContext);

    const firstOscillator = audioContext.createOscillator();
    const secondOscillator = audioContext.createOscillator();
    const firstGain = audioContext.createGain();
    const secondGain = audioContext.createGain();

    firstOscillator.type = 'sine';
    secondOscillator.type = 'sine';
    firstOscillator.frequency.value = 880;
    secondOscillator.frequency.value = 1174.66;

    firstOscillator.connect(firstGain);
    secondOscillator.connect(secondGain);
    firstGain.connect(audioContext.destination);
    secondGain.connect(audioContext.destination);

    const startTime = audioContext.currentTime;
    firstGain.gain.setValueAtTime(0, startTime);
    firstGain.gain.linearRampToValueAtTime(0.05, startTime + 0.02);
    firstGain.gain.linearRampToValueAtTime(0, startTime + 0.18);
    secondGain.gain.setValueAtTime(0, startTime + 0.08);
    secondGain.gain.linearRampToValueAtTime(0.05, startTime + 0.1);
    secondGain.gain.linearRampToValueAtTime(0, startTime + 0.26);
    firstOscillator.start(startTime);
    secondOscillator.start(startTime + 0.08);
    firstOscillator.stop(startTime + 0.22);
    secondOscillator.stop(startTime + 0.3);
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
