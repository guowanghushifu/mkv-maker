import { useEffect, useState } from 'react';
import { buildFilenamePreview, createApiClient } from './api/client';
import type { Draft, Job, ParsedBDInfo, SourceEntry } from './api/types';
import { Layout, type WorkflowStep } from './components/Layout';
import { LoginPage } from './features/auth/LoginPage';
import { BDInfoPage } from './features/bdinfo/BDInfoPage';
import { TrackEditorPage } from './features/draft/TrackEditorPage';
import { ReviewPage } from './features/review/ReviewPage';
import { ScanPage } from './features/sources/ScanPage';
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
  const [token, setToken] = useState<string | null>(null);
  const [step, setStep] = useState<WorkflowStep>('login');
  const [sources, setSources] = useState<SourceEntry[]>([]);
  const [selectedSourceId, setSelectedSourceId] = useState<string | null>(null);
  const [scanning, setScanning] = useState(false);
  const [bdinfoText, setBdinfoText] = useState('');
  const [parsedBDInfo, setParsedBDInfo] = useState<ParsedBDInfo | null>(null);
  const [bdinfoError, setBdinfoError] = useState<string | null>(null);
  const [scanError, setScanError] = useState<string | null>(null);
  const [loginError, setLoginError] = useState<string | null>(null);
  const [parsingBDInfo, setParsingBDInfo] = useState(false);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [filenamePreview, setFilenamePreview] = useState('');
  const [outputFilename, setOutputFilename] = useState('');
  const [filenameEdited, setFilenameEdited] = useState(false);
  const [submittingJob, setSubmittingJob] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [currentJob, setCurrentJob] = useState<Job | null>(null);
  const [currentJobLog, setCurrentJobLog] = useState('');

  const selectedSource = sources.find((source) => source.id === selectedSourceId) ?? null;
  const currentStep = token ? step : 'login';
  const fallbackTitle = draft?.title || parsedBDInfo?.discTitle || selectedSource?.name || 'Untitled';
  const outputPath = `${draft?.outputDir || '/output'}/${outputFilename || filenamePreview}`;

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

  const handleLogin = async (password: string) => {
    try {
      const loginResult = await api.login(password);
      setLoginError(null);
      setToken(loginResult.token);
      setStep('scan');
    } catch (error) {
      setLoginError(error instanceof Error ? error.message : 'Login failed.');
    }
  };

  const handleScan = async () => {
    setScanning(true);
    setScanError(null);
    try {
      const scannedSources = (await api.scanSources(token ?? undefined)).filter(
        (source) => source.type === 'bdmv'
      );
      setSources(scannedSources);
      if (scannedSources.length === 0) {
        setSelectedSourceId(null);
      } else if (!selectedSourceId || !scannedSources.some((source) => source.id === selectedSourceId)) {
        setSelectedSourceId(scannedSources[0].id);
      }
      if (step === 'bdinfo' && !scannedSources.some((source) => source.id === selectedSourceId)) {
        setStep('scan');
      }
    } catch (error) {
      setScanError(error instanceof Error ? error.message : 'Source scan failed.');
    } finally {
      setScanning(false);
    }
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
      setBdinfoError(error instanceof Error ? error.message : 'BDInfo parse failed.');
    } finally {
      setParsingBDInfo(false);
    }
  };

  const loadCurrentJob = async (): Promise<Job | null> => {
    const response = await fetch('/api/jobs/current', {
      method: 'GET',
      headers: token ? { Authorization: `Bearer ${token}` } : undefined,
    });
    if (response.status === 404) {
      return null;
    }
    if (!response.ok) {
      throw new Error(`Request failed with status ${response.status}`);
    }
    return (await response.json()) as Job;
  };

  const loadCurrentJobSnapshot = async () => {
    const [nextJob, nextLog] = await Promise.all([
      loadCurrentJob(),
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
      setSubmitError('A remux is already running. Please wait for it to finish.');
      return;
    }

    setSubmittingJob(true);
    setSubmitError(null);
    try {
      await api.submitJob(
        {
          source: selectedSource,
          bdinfo: parsedBDInfo,
          draft,
          outputFilename: finalFilename,
          outputPath: `${draft.outputDir || '/output'}/${finalFilename}`,
        },
        token ?? undefined
      );
      const { nextJob, nextLog } = await loadCurrentJobSnapshot();
      setCurrentJob(nextJob);
      setCurrentJobLog(nextLog);
      setStep('review');
    } catch (error) {
      setSubmitError(error instanceof Error ? error.message : 'Failed to start remux job.');
    } finally {
      setSubmittingJob(false);
    }
  };

  return (
    <Layout currentStep={currentStep}>
      {step === 'login' ? <LoginPage onSuccess={handleLogin} error={loginError} /> : null}

      {step === 'scan' ? (
        <ScanPage
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
          onSubmit={handleSubmitJob}
        />
      ) : null}
    </Layout>
  );
}

export default App;
