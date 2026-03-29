import { useState } from 'react';
import { createApiClient } from './api/client';
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
  const [jobs, setJobs] = useState<Job[]>([]);
  const [submittingJob, setSubmittingJob] = useState(false);

  const selectedSource = sources.find((source) => source.id === selectedSourceId) ?? null;
  const currentStep = token ? step : 'login';

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
  };

  const handleParseBDInfo = async () => {
    if (!selectedSource) {
      return;
    }

    setParsingBDInfo(true);
    setBdinfoError(null);
    try {
      const parsed = await api.parseBDInfo(bdinfoText, token ?? undefined);
      const nextDraft = await api.createDraft(selectedSource, parsed, token ?? undefined);
      setParsedBDInfo(parsed);
      setDraft(nextDraft);
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

    setSubmittingJob(true);
    try {
      await api.submitJob(
        {
          source: selectedSource,
          bdinfo: parsedBDInfo,
          draft,
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
          onChange={setDraft}
          onBack={() => setStep('bdinfo')}
          onNext={() => setStep('review')}
        />
      ) : null}

      {step === 'review' && selectedSource && parsedBDInfo && draft ? (
        <ReviewPage
          source={selectedSource}
          bdinfo={parsedBDInfo}
          draft={draft}
          submitting={submittingJob}
          onBack={() => setStep('editor')}
          onSubmit={handleSubmitJob}
        />
      ) : null}

      {step === 'jobs' ? (
        <JobsPage jobs={jobs} onStartNew={handleStartNewWorkflow} onRefresh={refreshJobs} />
      ) : null}
    </Layout>
  );
}

export default App;
