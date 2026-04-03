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
  try {
    const audioContext = getAudioContext();
    if (audioContext) {
      await resumeAudioContextIfSuspended(audioContext);
    }

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
    firstGain.gain.value = 0.05;
    secondGain.gain.value = 0.05;

    firstOscillator.connect(firstGain);
    secondOscillator.connect(secondGain);
    firstGain.connect(audioContext.destination);
    secondGain.connect(audioContext.destination);

    const startTime = audioContext.currentTime;
    firstOscillator.start(startTime);
    secondOscillator.start(startTime + 0.08);
    firstOscillator.stop(startTime + 0.18);
    secondOscillator.stop(startTime + 0.28);
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
