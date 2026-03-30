import { useEffect, useState } from 'react';
import { buildFilenamePreview, createApiClient } from './api/client';
import type { Draft, Job, ParsedBDInfo, SourceEntry } from './api/types';
import { Layout, type WorkflowStep } from './components/Layout';
import { LoginPage } from './features/auth/LoginPage';
import { BDInfoPage } from './features/bdinfo/BDInfoPage';
import { TrackEditorPage } from './features/draft/TrackEditorPage';
import { ReviewPage } from './features/review/ReviewPage';
import { ScanPage } from './features/sources/ScanPage';
import { getMessages, loadStoredLocale, loadStoredToken, saveStoredLocale, saveStoredToken, type Locale } from './i18n';
import { loadStoredWorkflowState, saveStoredWorkflowState } from './workflowState';
import './styles/app.css';

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

function App() {
  const [initialWorkflow] = useState(() => loadStoredWorkflowState());
  const [locale, setLocale] = useState<Locale>(() => loadStoredLocale());
  const [token, setToken] = useState<string | null>(() => loadStoredToken());
  const [step, setStep] = useState<WorkflowStep>(() =>
    loadStoredToken() ? initialWorkflow?.step ?? 'scan' : 'login'
  );
  const [sources, setSources] = useState<SourceEntry[]>(() => initialWorkflow?.sources ?? []);
  const [selectedSourceId, setSelectedSourceId] = useState<string | null>(() => initialWorkflow?.selectedSourceId ?? null);
  const [scanning, setScanning] = useState(false);
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
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [currentJob, setCurrentJob] = useState<Job | null>(null);
  const [currentJobLog, setCurrentJobLog] = useState('');

  const text = getMessages(locale);
  const selectedSource = sources.find((source) => source.id === selectedSourceId) ?? null;
  const currentStep = token ? step : 'login';
  const fallbackTitle = draft?.title || parsedBDInfo?.discTitle || selectedSource?.name || 'Untitled';
  const outputPath = `${draft?.outputDir || '/output'}/${outputFilename || filenamePreview}`;

  useEffect(() => {
    saveStoredLocale(locale);
  }, [locale]);

  useEffect(() => {
    saveStoredToken(token);
  }, [token]);

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

  useEffect(() => {
    if (!draft) {
      setFilenamePreview('');
      return;
    }

    let cancelled = false;
    const refreshFilename = async () => {
      const suggested = await api.previewFilename(draft, fallbackTitle, token ?? undefined);
      if (cancelled) {
        return;
      }
      setFilenamePreview(suggested);
      if (!filenameEdited) {
        setOutputFilename(suggested);
      }
    };
    void refreshFilename();

    return () => {
      cancelled = true;
    };
  }, [draft, fallbackTitle, filenameEdited, token]);

  const resetWorkflowState = () => {
    setSelectedSourceId(null);
    setBdinfoText('');
    setParsedBDInfo(null);
    setBdinfoError(null);
    setDraft(null);
    setFilenamePreview('');
    setOutputFilename('');
    setFilenameEdited(false);
    setSubmittingJob(false);
    setSubmitError(null);
    setCurrentJob(null);
    setCurrentJobLog('');
    setScanError(null);
  };

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
      const scannedSources = (await api.scanSources(token ?? undefined)).filter(
        (source) => source.type === 'bdmv'
      );
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
      setScanError(error instanceof Error ? error.message : text.app.scanFailed);
    } finally {
      setScanning(false);
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
      setBdinfoError(error instanceof Error ? error.message : text.app.bdinfoParseFailed);
    } finally {
      setParsingBDInfo(false);
    }
  };

  const loadCurrentJobSnapshot = async () => {
    const [nextJob, nextLog] = await Promise.all([
      api.currentJob(token ?? undefined),
      api.currentJobLog(token ?? undefined),
    ]);
    return { nextJob, nextLog: nextJob ? nextLog : '' };
  };

  const refreshCurrentJob = async () => {
    try {
      const { nextJob, nextLog } = await loadCurrentJobSnapshot();
      setCurrentJob(nextJob);
      setCurrentJobLog(nextLog);
    } catch {
      // Keep current snapshot when polling fails to avoid disrupting review flow.
    }
  };

  useEffect(() => {
    if (!token) {
      return;
    }
    void refreshCurrentJob();
  }, [token]);

  useEffect(() => {
    if (!currentJob || currentJob.status !== 'running') {
      return;
    }

    const interval = window.setInterval(() => {
      void refreshCurrentJob();
    }, 1500);

    return () => {
      window.clearInterval(interval);
    };
  }, [currentJob, token]);

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

    setSubmittingJob(true);
    setSubmitError(null);
    setCurrentJob(null);
    setCurrentJobLog('');
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
      setCurrentJob(startedJob);
      setCurrentJobLog('');
      setStep('review');

      const { nextJob, nextLog } = await loadCurrentJobSnapshot();
      if (nextJob && nextJob.id === startedJob.id) {
        setCurrentJob(nextJob);
        setCurrentJobLog(nextLog);
      }
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : text.app.submitFailed);
    } finally {
      setSubmittingJob(false);
    }
  };

  return (
    <Layout
      currentStep={currentStep}
      locale={locale}
      onToggleLocale={() => setLocale((current) => (current === 'zh' ? 'en' : 'zh'))}
    >
      {step === 'login' ? <LoginPage locale={locale} onSuccess={handleLogin} error={loginError} /> : null}

      {step === 'scan' ? (
        <ScanPage
          locale={locale}
          loading={scanning}
          error={scanError}
          sources={sources}
          selectedSourceId={selectedSourceId}
          onScan={handleScan}
          onSelectSource={handleSourceSelect}
          onNext={() => setStep('bdinfo')}
        />
      ) : null}

      {step === 'bdinfo' && selectedSource ? (
        <BDInfoPage
          locale={locale}
          source={selectedSource}
          bdinfoText={bdinfoText}
          parsed={parsedBDInfo}
          error={bdinfoError}
          loading={parsingBDInfo}
          onBack={() => setStep('scan')}
          onTextChange={setBdinfoText}
          onSubmit={handleParseBDInfo}
        />
      ) : null}

      {step === 'editor' && draft ? (
        <TrackEditorPage
          locale={locale}
          draft={draft}
          filenamePreview={filenamePreview}
          outputFilename={outputFilename}
          onFilenameChange={(value) => {
            setFilenameEdited(true);
            setOutputFilename(value);
          }}
          onChange={(next) => setDraft(normalizeDraft(next))}
          onBack={() => setStep('bdinfo')}
          onNext={() => setStep('review')}
        />
      ) : null}

      {step === 'review' && selectedSource && parsedBDInfo && draft ? (
        <ReviewPage
          locale={locale}
          source={selectedSource}
          bdinfo={parsedBDInfo}
          draft={draft}
          outputFilename={outputFilename || filenamePreview}
          outputPath={outputPath}
          submitting={submittingJob}
          startDisabled={currentJob?.status === 'running'}
          submitError={submitError}
          currentJob={currentJob}
          currentLog={currentJobLog}
          onBack={() => setStep('editor')}
          onStartNextRemux={handleStartNextRemux}
          onSubmit={handleSubmitJob}
        />
      ) : null}
    </Layout>
  );
}

export default App;
