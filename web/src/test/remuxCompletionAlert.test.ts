import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  prepareRemuxCompletionAlerts,
  playRemuxCompletionChime,
  showRemuxCompletionNotification,
} from '../remuxCompletionAlert';

afterEach(() => {
  vi.unstubAllGlobals();
  vi.clearAllMocks();
  vi.restoreAllMocks();
});

describe('prepareRemuxCompletionAlerts', () => {
  it('requests notification permission during preparation when permission is still default', async () => {
    const resume = vi.fn().mockResolvedValue(undefined);
    const requestPermission = vi.fn().mockResolvedValue('granted');
    const AudioContextMock = vi.fn(function () {
      return {
        state: 'suspended',
        resume,
      };
    });

    vi.stubGlobal('AudioContext', AudioContextMock);
    vi.stubGlobal('Notification', {
      permission: 'default',
      requestPermission,
    });

    await prepareRemuxCompletionAlerts();

    expect(resume).toHaveBeenCalledTimes(1);
    expect(requestPermission).toHaveBeenCalledTimes(1);
  });

  it('requests notification permission without waiting for a slow audio resume', async () => {
    let resolveResume: (() => void) | null = null;
    const resume = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          resolveResume = resolve;
        }),
    );
    const requestPermission = vi.fn().mockResolvedValue('granted');
    const AudioContextMock = vi.fn(function () {
      return {
        state: 'suspended',
        resume,
      };
    });

    vi.stubGlobal('AudioContext', AudioContextMock);
    vi.stubGlobal('Notification', {
      permission: 'default',
      requestPermission,
    });

    const preparePromise = prepareRemuxCompletionAlerts();
    await Promise.resolve();

    expect(resume).toHaveBeenCalledTimes(1);
    expect(requestPermission).toHaveBeenCalledTimes(1);

    resolveResume?.();
    await preparePromise;
  });

  it('still requests notification permission when audio resume fails', async () => {
    const resume = vi.fn().mockRejectedValue(new Error('resume failed'));
    const requestPermission = vi.fn().mockResolvedValue('granted');
    const AudioContextMock = vi.fn(function () {
      return {
        state: 'suspended',
        resume,
      };
    });

    vi.stubGlobal('AudioContext', AudioContextMock);
    vi.stubGlobal('Notification', {
      permission: 'default',
      requestPermission,
    });

    await prepareRemuxCompletionAlerts();

    expect(resume).toHaveBeenCalledTimes(1);
    expect(requestPermission).toHaveBeenCalledTimes(1);
  });

  it('does not leak an unhandled rejection when audio warm-up fails after notification early return', async () => {
    const unhandledRejections: unknown[] = [];
    const onUnhandledRejection = (event: PromiseRejectionEvent) => {
      unhandledRejections.push(event.reason);
      event.preventDefault();
    };
    window.addEventListener('unhandledrejection', onUnhandledRejection);

    const resume = vi.fn().mockRejectedValue(new Error('resume failed'));
    const AudioContextMock = vi.fn(function () {
      return {
        state: 'suspended',
        resume,
      };
    });

    vi.stubGlobal('AudioContext', AudioContextMock);
    vi.stubGlobal('Notification', {
      permission: 'granted',
      requestPermission: vi.fn(),
    });

    try {
      await prepareRemuxCompletionAlerts();
      await Promise.resolve();
      await Promise.resolve();
    } finally {
      window.removeEventListener('unhandledrejection', onUnhandledRejection);
    }

    expect(resume).toHaveBeenCalledTimes(1);
    expect(unhandledRejections).toEqual([]);
  });
});

describe('playRemuxCompletionChime', () => {
  it('creates two oscillator tones for the completion chime when audio is available', async () => {
    const resume = vi.fn().mockResolvedValue(undefined);
    const oscillatorOne = {
      frequency: { value: 0 },
      connect: vi.fn(),
      start: vi.fn(),
      stop: vi.fn(),
    };
    const oscillatorTwo = {
      frequency: { value: 0 },
      connect: vi.fn(),
      start: vi.fn(),
      stop: vi.fn(),
    };
    const gainOne = {
      gain: {
        setValueAtTime: vi.fn(),
        linearRampToValueAtTime: vi.fn(),
      },
      connect: vi.fn(),
    };
    const gainTwo = {
      gain: {
        setValueAtTime: vi.fn(),
        linearRampToValueAtTime: vi.fn(),
      },
      connect: vi.fn(),
    };
    const destination = {};
    const AudioContextMock = vi.fn(function () {
      return {
        state: 'suspended',
        resume,
        currentTime: 0,
        destination,
        createOscillator: vi.fn().mockReturnValueOnce(oscillatorOne).mockReturnValueOnce(oscillatorTwo),
        createGain: vi.fn().mockReturnValueOnce(gainOne).mockReturnValueOnce(gainTwo),
      };
    });

    vi.stubGlobal('AudioContext', AudioContextMock);

    await playRemuxCompletionChime();

    expect(AudioContextMock).toHaveBeenCalledTimes(1);
    expect(resume).toHaveBeenCalledTimes(1);
    expect(oscillatorOne.frequency.value).toBe(880);
    expect(oscillatorTwo.frequency.value).toBe(1174.66);
    expect(oscillatorOne.connect).toHaveBeenCalledTimes(1);
    expect(oscillatorTwo.connect).toHaveBeenCalledTimes(1);
    expect(gainOne.connect).toHaveBeenCalledTimes(1);
    expect(gainTwo.connect).toHaveBeenCalledTimes(1);
    expect(gainOne.gain.setValueAtTime).toHaveBeenCalledWith(0, 0);
    expect(gainOne.gain.linearRampToValueAtTime).toHaveBeenCalledWith(0.05, 0.02);
    expect(gainOne.gain.linearRampToValueAtTime).toHaveBeenCalledWith(0, 0.18);
    expect(gainTwo.gain.setValueAtTime).toHaveBeenCalledWith(0, 0.08);
    expect(gainTwo.gain.linearRampToValueAtTime).toHaveBeenCalledWith(0.05, 0.1);
    expect(gainTwo.gain.linearRampToValueAtTime).toHaveBeenCalledWith(0, 0.26);
    expect(oscillatorOne.start).toHaveBeenCalledWith(0);
    expect(oscillatorTwo.start).toHaveBeenCalledWith(0.08);
    expect(oscillatorOne.stop).toHaveBeenCalledWith(0.22);
    expect(oscillatorTwo.stop).toHaveBeenCalledWith(0.3);
    expect(oscillatorOne.stop).toHaveBeenCalledTimes(1);
    expect(oscillatorTwo.stop).toHaveBeenCalledTimes(1);
    expect(oscillatorOne.start).toHaveBeenCalledTimes(1);
    expect(oscillatorTwo.start).toHaveBeenCalledTimes(1);
  });
});

describe('showRemuxCompletionNotification', () => {
  it('does nothing when notifications are unsupported', () => {
    vi.stubGlobal('Notification', undefined);

    expect(() =>
      showRemuxCompletionNotification({
        title: 'Remux complete',
        body: 'Your MKV is ready.',
      }),
    ).not.toThrow();
  });

  it('does nothing when notification permission is denied', () => {
    const NotificationMock = vi.fn();
    NotificationMock.permission = 'denied';

    vi.stubGlobal('Notification', NotificationMock);

    showRemuxCompletionNotification({
      title: 'Remux complete',
      body: 'Your MKV is ready.',
    });

    expect(NotificationMock).not.toHaveBeenCalled();
  });

  it('shows a notification and focuses the window when the notification is clicked', () => {
    const close = vi.fn();
    let notificationInstance: { onclick: null | (() => void); close: () => void } | null = null;
    const NotificationMock = vi.fn(function () {
      notificationInstance = {
        onclick: null,
        close,
      };
      return notificationInstance;
    });
    NotificationMock.permission = 'granted';
    const focus = vi.spyOn(window, 'focus').mockImplementation(() => undefined);

    vi.stubGlobal('Notification', NotificationMock);

    showRemuxCompletionNotification({
      title: 'Remux complete',
      body: 'Your MKV is ready.',
    });

    expect(NotificationMock).toHaveBeenCalledTimes(1);
    expect(NotificationMock).toHaveBeenCalledWith('Remux complete', {
      body: 'Your MKV is ready.',
      tag: 'mkv-maker-remux-complete',
    });
    expect(notificationInstance).not.toBeNull();

    notificationInstance?.onclick?.();

    expect(focus).toHaveBeenCalledTimes(1);
    expect(close).toHaveBeenCalledTimes(1);
  });
});
