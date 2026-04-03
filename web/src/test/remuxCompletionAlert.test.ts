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
  it('creates a louder five-repeat completion chime when audio is available', async () => {
    const resume = vi.fn().mockResolvedValue(undefined);
    const oscillators = Array.from({ length: 10 }, () => ({
      frequency: { value: 0 },
      connect: vi.fn(),
      start: vi.fn(),
      stop: vi.fn(),
    }));
    const gains = Array.from({ length: 10 }, () => ({
      gain: {
        setValueAtTime: vi.fn(),
        linearRampToValueAtTime: vi.fn(),
      },
      connect: vi.fn(),
    }));
    const destination = {};
    let oscillatorIndex = 0;
    let gainIndex = 0;
    const AudioContextMock = vi.fn(function () {
      return {
        state: 'suspended',
        resume,
        currentTime: 0,
        destination,
        createOscillator: vi.fn(() => oscillators[oscillatorIndex++]),
        createGain: vi.fn(() => gains[gainIndex++]),
      };
    });

    vi.stubGlobal('AudioContext', AudioContextMock);

    await playRemuxCompletionChime();

    expect(AudioContextMock).toHaveBeenCalledTimes(1);
    expect(resume).toHaveBeenCalledTimes(1);
    expect(oscillators).toHaveLength(10);
    expect(gains).toHaveLength(10);

    const expectedStarts = [0, 0.08, 0.55, 0.63, 1.1, 1.18, 1.65, 1.73, 2.2, 2.28];
    const expectedStops = [0.22, 0.3, 0.77, 0.85, 1.32, 1.4, 1.87, 1.95, 2.42, 2.5];
    const expectedFadeOuts = [0.18, 0.26, 0.73, 0.81, 1.28, 1.36, 1.83, 1.91, 2.38, 2.46];

    oscillators.forEach((oscillator, index) => {
      expect(oscillator.frequency.value).toBe(index % 2 === 0 ? 880 : 1174.66);
      expect(oscillator.connect).toHaveBeenCalledTimes(1);
      expect(oscillator.start).toHaveBeenCalledTimes(1);
      expect(oscillator.stop).toHaveBeenCalledTimes(1);
      expect(oscillator.start.mock.calls[0]?.[0]).toBeCloseTo(expectedStarts[index], 6);
      expect(oscillator.stop.mock.calls[0]?.[0]).toBeCloseTo(expectedStops[index], 6);
    });

    gains.forEach((gainNode, index) => {
      expect(gainNode.connect).toHaveBeenCalledTimes(1);
      expect(gainNode.gain.setValueAtTime).toHaveBeenCalledTimes(1);
      expect(gainNode.gain.linearRampToValueAtTime).toHaveBeenCalledTimes(2);
      expect(gainNode.gain.setValueAtTime.mock.calls[0]?.[0]).toBe(0);
      expect(gainNode.gain.setValueAtTime.mock.calls[0]?.[1]).toBeCloseTo(expectedStarts[index], 6);
      expect(gainNode.gain.linearRampToValueAtTime.mock.calls[0]?.[0]).toBe(0.18);
      expect(gainNode.gain.linearRampToValueAtTime.mock.calls[0]?.[1]).toBeCloseTo(
        expectedStarts[index] + 0.02,
        6,
      );
      expect(gainNode.gain.linearRampToValueAtTime.mock.calls[1]?.[0]).toBe(0);
      expect(gainNode.gain.linearRampToValueAtTime.mock.calls[1]?.[1]).toBeCloseTo(expectedFadeOuts[index], 6);
    });
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
