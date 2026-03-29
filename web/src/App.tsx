import { useEffect, useState } from 'react';
import { buildFilenamePreview, createApiClient } from './api/client';
import type { Draft, Job, ParsedBDInfo, SourceEntry } from './api/types';
import { Layout, type WorkflowStep } from './components/Layout';
import { LoginPage } from './features/auth/LoginPage';
import { BDInfoPage } from './features/bdinfo/BDInfoPage';
import { TrackEditorPage } from './features/draft/TrackEditorPage';
import { JobsPage } from './features/jobs/JobsPage';
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
  const [parsingBDInfo, setParsingBDInfo] = useState(false);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [filenamePreview, setFilenamePreview] = useState('');
  const [outputFilename, setOutputFilename] = useState('');
  const [filenameEdited, setFilenameEdited] = useState(false);
  const [jobs, setJobs] = useState<Job[]>([]);
  const [submittingJob, setSubmittingJob] = useState(false);

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
    const loginResult = await api.login(password);
    setToken(loginResult.token);
    setStep('scan');
  };

  const handleScan = async () => {
    setScanning(true);
    try {
      const scannedSources = (await api.scanSources(token ?? undefined)).filter(
        (source) => source.type === 'bdmv'
      );
      setSources(scannedSources);
      if (!selectedSourceId && scannedSources.length > 0) {
        setSelectedSourceId(scannedSources[0].id);
      }
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

  const refreshJobs = async () => {
    const nextJobs = await api.listJobs(token ?? undefined);
    setJobs(nextJobs);
  };

  const handleSubmitJob = async () => {
    if (!selectedSource || !parsedBDInfo || !draft) {
      return;
    }

    const finalFilename = (outputFilename || filenamePreview).trim();
    if (!finalFilename) {
      return;
    }

    setSubmittingJob(true);
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
      await refreshJobs();
      setStep('jobs');
    } finally {
      setSubmittingJob(false);
    }
  };

  const handleStartNewWorkflow = () => {
    setStep('scan');
    setSelectedSourceId(null);
    setBdinfoText('');
    setParsedBDInfo(null);
    setBdinfoError(null);
    setDraft(null);
    setOutputFilename('');
    setFilenamePreview('');
    setFilenameEdited(false);
  };

  return (
    <Layout currentStep={currentStep}>
      {step === 'login' ? <LoginPage onSuccess={handleLogin} /> : null}

      {step === 'scan' ? (
        <ScanPage
          loading={scanning}
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
          onBack={() => setStep('editor')}
          onSubmit={handleSubmitJob}
        />
      ) : null}

      {step === 'jobs' ? (
        <JobsPage
          jobs={jobs}
          onStartNew={handleStartNewWorkflow}
          onRefresh={refreshJobs}
          onLoadLog={(jobId) => api.getJobLog(jobId, token ?? undefined)}
        />
      ) : null}
    </Layout>
  );
}

export default App;

