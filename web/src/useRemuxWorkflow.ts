import { useEffect, useRef, useState } from 'react';
import { UnauthorizedError, buildFilenamePreview, createApiClient } from './api/client';
import type { Draft, Job, ParsedBDInfo, SourceEntry } from './api/types';
import type { WorkflowStep } from './components/Layout';
import {
  getMessages,
  loadStoredLocale,
  loadStoredToken,
  saveStoredLocale,
  saveStoredToken,
  type Locale,
} from './i18n';
import {
  playRemuxCompletionChime,
  prepareRemuxCompletionAlerts,
  showRemuxCompletionNotification,
} from './remuxCompletionAlert';
import { loadStoredWorkflowState, saveStoredWorkflowState } from './workflowState';

const api = createApiClient();

function normalizeDraft(nextDraft: Draft): Draft {
  return {
    ...nextDraft,
    outputDir: nextDraft.outputDir || '/output',
    dvMergeEnabled:
      typeof nextDraft.dvMergeEnabled === 'boolean'
        ? nextDraft.dvMergeEnabled
        : (nextDraft.video.hdrType || '').toUpperCase().includes('DV'),
  };
}

export function useRemuxWorkflow() {
  const [initialWorkflow] = useState(() => loadStoredWorkflowState());
  const [locale, setLocale] = useState<Locale>(() => loadStoredLocale());
  const [token, setToken] = useState<string | null>(() => loadStoredToken());
  const [step, setStep] = useState<WorkflowStep>(() =>
    loadStoredToken() ? initialWorkflow?.step ?? 'scan' : 'login'
  );
  const [sources, setSources] = useState<SourceEntry[]>(() => initialWorkflow?.sources ?? []);
  const [selectedSourceId, setSelectedSourceId] = useState<string | null>(() => initialWorkflow?.selectedSourceId ?? null);
  const [scanning, setScanning] = useState(false);
  const [releasingMountedISOs, setReleasingMountedISOs] = useState(false);
  const [bdinfoText, setBdinfoText] = useState(() => initialWorkflow?.bdinfoText ?? '');
  const [parsedBDInfo, setParsedBDInfo] = useState<ParsedBDInfo | null>(() => initialWorkflow?.parsedBDInfo ?? null);
  const [bdinfoError, setBdinfoError] = useState<string | null>(null);
  const [scanError, setScanError] = useState<string | null>(null);
  const [loginError, setLoginError] = useState<string | null>(null);
  const [parsingBDInfo, setParsingBDInfo] = useState(false);
  const [draft, setDraft] = useState<Draft | null>(() => initialWorkflow?.draft ?? null);
  const [filenamePreview, setFilenamePreview] = useState(() => initialWorkflow?.filenamePreview ?? '');
  const [outputFilename, setOutputFilename] = useState(() => initialWorkflow?.outputFilename ?? '');
  const [filenameEdited, setFilenameEdited] = useState(() => initialWorkflow?.filenameEdited ?? false);
  const [submittingJob, setSubmittingJob] = useState(false);
  const [stoppingJob, setStoppingJob] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [currentJob, setCurrentJob] = useState<Job | null>(null);
  const [currentJobLog, setCurrentJobLog] = useState('');
  const currentJobSnapshotRequestRef = useRef(0);
  const armedCompletionJobIdRef = useRef<string | null>(null);
  const alertedCompletionJobIdRef = useRef<string | null>(null);
  const previousSeenJobIdRef = useRef<string | null>(null);
  const previousSeenJobStatusRef = useRef<Job['status'] | null>(null);
  const completionAlertGenerationRef = useRef(0);

  const text = getMessages(locale);
  const latestMessagesRef = useRef(text);
  latestMessagesRef.current = text;
  const selectedSource = sources.find((source) => source.id === selectedSourceId) ?? null;
  const currentStep = token ? step : 'login';
  const fallbackTitle = draft?.title || parsedBDInfo?.discTitle || selectedSource?.name || 'Untitled';
  const outputPath = `${draft?.outputDir || '/output'}/${outputFilename || filenamePreview}`;
  const layoutContext = {
    source: selectedSource?.name || text.layout.pendingValue,
    playlist: parsedBDInfo?.playlistName || text.layout.pendingValue,
    output: outputFilename || filenamePreview || text.layout.pendingValue,
    task: currentJob ? text.status[currentJob.status] : token ? text.layout.readyState : text.layout.lockedState,
  };

  useEffect(() => {
    saveStoredLocale(locale);
  }, [locale]);

  useEffect(() => {
    saveStoredToken(token);
  }, [token]);

  const resetCompletionAlertState = () => {
    completionAlertGenerationRef.current += 1;
    armedCompletionJobIdRef.current = null;
    alertedCompletionJobIdRef.current = null;
    previousSeenJobIdRef.current = null;
    previousSeenJobStatusRef.current = null;
  };

  useEffect(() => {
    if (!token) {
      saveStoredWorkflowState(null);
      return;
    }
    saveStoredWorkflowState({
      step,
      sources,
      selectedSourceId,
      bdinfoText,
      parsedBDInfo,
      draft,
      filenamePreview,
      outputFilename,
      filenameEdited,
    });
  }, [token, step, sources, selectedSourceId, bdinfoText, parsedBDInfo, draft, filenamePreview, outputFilename, filenameEdited]);

  const resetWorkflowState = () => {
    invalidateCurrentJobSnapshots();
    resetCompletionAlertState();
    setSelectedSourceId(null);
    setBdinfoText('');
    setParsedBDInfo(null);
    setBdinfoError(null);
    setDraft(null);
    setFilenamePreview('');
    setOutputFilename('');
    setFilenameEdited(false);
    setSubmittingJob(false);
    setStoppingJob(false);
    setSubmitError(null);
    setCurrentJob(null);
    setCurrentJobLog('');
    setScanError(null);
    setReleasingMountedISOs(false);
  };

  const handleUnauthorized = () => {
    resetWorkflowState();
    setSources([]);
    setLoginError(null);
    setToken(null);
    setStep('login');
  };

  const invalidateCurrentJobSnapshots = () => {
    currentJobSnapshotRequestRef.current += 1;
  };

  useEffect(() => {
    if (!draft) {
      setFilenamePreview('');
      return;
    }

    let cancelled = false;
    const refreshFilename = async () => {
      try {
        const suggested = await api.previewFilename(draft, fallbackTitle, token ?? undefined);
        if (cancelled) {
          return;
        }
        setFilenamePreview(suggested);
        if (!filenameEdited) {
          setOutputFilename(suggested);
        }
      } catch (error) {
        if (cancelled) {
          return;
        }
        if (error instanceof UnauthorizedError) {
          handleUnauthorized();
        }
      }
    };
    void refreshFilename();

    return () => {
      cancelled = true;
    };
  }, [draft, fallbackTitle, filenameEdited, token]);

  const handleLogin = async (password: string) => {
    try {
      const loginResult = await api.login(password);
      setLoginError(null);
      setToken(loginResult.token);
      setStep('scan');
    } catch (error) {
      setLoginError(error instanceof Error ? error.message : text.app.loginFailed);
    }
  };

  const handleScan = async (preserveSelection = true) => {
    setScanning(true);
    setScanError(null);
    try {
      const scannedSources = await api.scanSources(token ?? undefined);
      setSources(scannedSources);
      if (scannedSources.length === 0) {
        setSelectedSourceId(null);
      } else if (!preserveSelection) {
        setSelectedSourceId(null);
      } else if (!selectedSourceId || !scannedSources.some((source) => source.id === selectedSourceId)) {
        setSelectedSourceId(scannedSources[0].id);
      }
      if (step === 'bdinfo' && !scannedSources.some((source) => source.id === selectedSourceId)) {
        setStep('scan');
      }
    } catch (error) {
      if (error instanceof UnauthorizedError) {
        handleUnauthorized();
        return;
      }
      setScanError(error instanceof Error ? error.message : text.app.scanFailed);
    } finally {
      setScanning(false);
    }
  };

  const handleReleaseMountedISOs = async () => {
    setReleasingMountedISOs(true);
    try {
      const result = await api.releaseMountedISOs(token ?? undefined);
      if (result.failed > 0 || result.skippedInUse > 0) {
        setScanError(text.app.releaseMountedISOsPartial(result.released, result.skippedInUse, result.failed));
      } else {
        setScanError(null);
      }
    } catch (error) {
      if (error instanceof UnauthorizedError) {
        handleUnauthorized();
        return;
      }
      setScanError(text.app.releaseMountedISOsFailed);
    } finally {
      setReleasingMountedISOs(false);
    }
  };

  const handleStartNextRemux = async () => {
    resetWorkflowState();
    setStep('scan');
    await handleScan(false);
  };

  const handleSourceSelect = (sourceId: string) => {
    setSelectedSourceId(sourceId);
    setParsedBDInfo(null);
    setBdinfoText('');
    setBdinfoError(null);
    setDraft(null);
    setOutputFilename('');
    setFilenamePreview('');
    setFilenameEdited(false);
  };

  const handleParseBDInfo = async () => {
    if (!selectedSource) {
      return;
    }

    setParsingBDInfo(true);
    setBdinfoError(null);
    try {
      const parsed = await api.parseBDInfo(bdinfoText, token ?? undefined);
      const nextDraft = normalizeDraft(await api.createDraft(selectedSource, parsed, token ?? undefined));
      const localPreview = buildFilenamePreview(nextDraft, parsed.discTitle || selectedSource.name);
      setParsedBDInfo(parsed);
      setDraft(nextDraft);
      setFilenamePreview(localPreview);
      setOutputFilename(localPreview);
      setFilenameEdited(false);
      setStep('editor');
    } catch (error) {
      if (error instanceof UnauthorizedError) {
        handleUnauthorized();
        return;
      }
      setBdinfoError(error instanceof Error ? error.message : text.app.bdinfoParseFailed);
    } finally {
      setParsingBDInfo(false);
    }
  };

  const loadCurrentJobSnapshot = async () => {
    const requestId = currentJobSnapshotRequestRef.current + 1;
    currentJobSnapshotRequestRef.current = requestId;
    const nextJob = await api.currentJob(token ?? undefined);
    if (!nextJob) {
      return { requestId, nextJob, nextLog: '' };
    }
    let nextLog = await api.currentJobLog(token ?? undefined);
    if (nextJob.status !== 'running') {
      nextLog = await api.currentJobLog(token ?? undefined);
    }
    return { requestId, nextJob, nextLog: nextJob ? nextLog : '' };
  };

  const applyCurrentJobSnapshot = ({
    requestId,
    nextJob,
    nextLog,
  }: {
    requestId: number;
    nextJob: Job | null;
    nextLog: string;
  }) => {
    if (requestId !== currentJobSnapshotRequestRef.current) {
      return false;
    }

    const previousJobId = previousSeenJobIdRef.current;
    const previousJobStatus = previousSeenJobStatusRef.current;
    const nextJobId = nextJob?.id ?? null;
    const nextJobStatus = nextJob?.status ?? null;

    if (
      nextJob &&
      armedCompletionJobIdRef.current === nextJob.id &&
      alertedCompletionJobIdRef.current !== nextJob.id &&
      previousJobId === nextJob.id &&
      previousJobStatus === 'running' &&
      nextJob.status === 'succeeded'
    ) {
      alertedCompletionJobIdRef.current = nextJob.id;
      const completionAlertGeneration = completionAlertGenerationRef.current;
      void playRemuxCompletionChime().catch(() => undefined);
      if (
        completionAlertGenerationRef.current !== completionAlertGeneration ||
        alertedCompletionJobIdRef.current !== nextJob.id
      ) {
        return false;
      }
      const latestMessages = latestMessagesRef.current;
      showRemuxCompletionNotification({
        title: latestMessages.review.remuxCompletedNotificationTitle,
        body: latestMessages.review.remuxCompletedNotificationBody(nextJob.outputName),
      });
    }

    previousSeenJobIdRef.current = nextJobId;
    previousSeenJobStatusRef.current = nextJobStatus;
    setCurrentJob(nextJob);
    setCurrentJobLog(nextLog);
    return true;
  };

  const refreshCurrentJob = async () => {
    try {
      const snapshot = await loadCurrentJobSnapshot();
      applyCurrentJobSnapshot(snapshot);
    } catch (error) {
      if (error instanceof UnauthorizedError) {
        handleUnauthorized();
        return;
      }
    }
  };

  useEffect(() => {
    if (!token) {
      return;
    }
    void refreshCurrentJob();
  }, [token]);

  useEffect(() => {
    if (!currentJob || currentJob.status !== 'running' || submittingJob || stoppingJob) {
      return;
    }

    const interval = window.setInterval(() => {
      void refreshCurrentJob();
    }, 1500);

    return () => {
      window.clearInterval(interval);
    };
  }, [currentJob, submittingJob, stoppingJob, token]);

  const handleSubmitJob = async () => {
    if (!selectedSource || !parsedBDInfo || !draft) {
      return;
    }

    const finalFilename = (outputFilename || filenamePreview).trim();
    if (!finalFilename) {
      return;
    }
    if (currentJob?.status === 'running') {
      setSubmitError(text.app.currentJobRunning);
      return;
    }

    invalidateCurrentJobSnapshots();
    setSubmittingJob(true);
    setSubmitError(null);
    setCurrentJob(null);
    setCurrentJobLog('');
    void prepareRemuxCompletionAlerts().catch(() => undefined);
    try {
      const startedJob = await api.submitJob(
        {
          source: selectedSource,
          bdinfo: parsedBDInfo,
          draft,
          outputFilename: finalFilename,
          outputPath: `${draft.outputDir || '/output'}/${finalFilename}`,
        },
        token ?? undefined
      );
      armedCompletionJobIdRef.current = startedJob.id;
      alertedCompletionJobIdRef.current = null;
      previousSeenJobIdRef.current = startedJob.id;
      previousSeenJobStatusRef.current = startedJob.status;
      setCurrentJob(startedJob);
      setCurrentJobLog('');
      setStep('review');

      const snapshot = await loadCurrentJobSnapshot();
      if (snapshot.nextJob && snapshot.nextJob.id === startedJob.id) {
        applyCurrentJobSnapshot(snapshot);
      }
    } catch (error) {
      if (error instanceof UnauthorizedError) {
        handleUnauthorized();
        return;
      }
      setSubmitError(error instanceof Error ? error.message : text.app.submitFailed);
    } finally {
      setSubmittingJob(false);
    }
  };

  const handleStopCurrentJob = async () => {
    if (!currentJob || currentJob.status !== 'running' || submittingJob || stoppingJob) {
      return;
    }

    invalidateCurrentJobSnapshots();
    setStoppingJob(true);
    setSubmitError(null);
    try {
      await api.stopCurrentJob(token ?? undefined);
      const snapshot = await loadCurrentJobSnapshot();
      applyCurrentJobSnapshot(snapshot);
    } catch (error) {
      if (error instanceof UnauthorizedError) {
        handleUnauthorized();
        return;
      }
      try {
        const snapshot = await loadCurrentJobSnapshot();
        const applied = applyCurrentJobSnapshot(snapshot);
        if (applied && snapshot.nextJob && snapshot.nextJob.status !== 'running') {
          setSubmitError(null);
          return;
        }
      } catch (refreshError) {
        if (refreshError instanceof UnauthorizedError) {
          handleUnauthorized();
          return;
        }
      }
      setSubmitError(text.app.stopFailed);
    } finally {
      setStoppingJob(false);
    }
  };

  return {
    locale,
    step,
    currentStep,
    token,
    sources,
    selectedSourceId,
    selectedSource,
    scanning,
    releasingMountedISOs,
    bdinfoText,
    parsedBDInfo,
    bdinfoError,
    scanError,
    loginError,
    parsingBDInfo,
    draft,
    filenamePreview,
    outputFilename,
    submittingJob,
    stoppingJob,
    submitError,
    currentJob,
    currentJobLog,
    outputPath,
    layoutContext,
    toggleLocale: () => setLocale((current) => (current === 'zh' ? 'en' : 'zh')),
    goToStep: (nextStep: WorkflowStep) => setStep(nextStep),
    setBdinfoText,
    updateOutputFilename: (value: string) => {
      setFilenameEdited(true);
      setOutputFilename(value);
    },
    updateDraft: (nextDraft: Draft) => {
      setDraft(normalizeDraft(nextDraft));
    },
    handleLogin,
    handleReleaseMountedISOs,
    handleScan,
    handleSourceSelect,
    handleParseBDInfo,
    handleSubmitJob,
    handleStopCurrentJob,
    handleStartNextRemux,
  };
}
