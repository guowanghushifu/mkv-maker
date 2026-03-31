import { Layout } from './components/Layout';
import { LoginPage } from './features/auth/LoginPage';
import { BDInfoPage } from './features/bdinfo/BDInfoPage';
import { TrackEditorPage } from './features/draft/TrackEditorPage';
import { ReviewPage } from './features/review/ReviewPage';
import { ScanPage } from './features/sources/ScanPage';
import { useRemuxWorkflow } from './useRemuxWorkflow';

function App() {
  const workflow = useRemuxWorkflow();

  if (workflow.step === 'login') {
    return <LoginPage locale={workflow.locale} onSuccess={workflow.handleLogin} error={workflow.loginError} />;
  }

  return (
    <Layout
      currentStep={workflow.currentStep}
      locale={workflow.locale}
      onToggleLocale={workflow.toggleLocale}
      context={workflow.layoutContext}
    >
      {workflow.step === 'scan' ? (
        <ScanPage
          locale={workflow.locale}
          loading={workflow.scanning}
          error={workflow.scanError}
          sources={workflow.sources}
          selectedSourceId={workflow.selectedSourceId}
          onScan={workflow.handleScan}
          onSelectSource={workflow.handleSourceSelect}
          onNext={() => workflow.goToStep('bdinfo')}
        />
      ) : null}

      {workflow.step === 'bdinfo' && workflow.selectedSource ? (
        <BDInfoPage
          locale={workflow.locale}
          source={workflow.selectedSource}
          bdinfoText={workflow.bdinfoText}
          parsed={workflow.parsedBDInfo}
          error={workflow.bdinfoError}
          loading={workflow.parsingBDInfo}
          onBack={() => workflow.goToStep('scan')}
          onTextChange={workflow.setBdinfoText}
          onSubmit={workflow.handleParseBDInfo}
        />
      ) : null}

      {workflow.step === 'editor' && workflow.draft ? (
        <TrackEditorPage
          locale={workflow.locale}
          draft={workflow.draft}
          filenamePreview={workflow.filenamePreview}
          outputFilename={workflow.outputFilename}
          onFilenameChange={workflow.updateOutputFilename}
          onChange={workflow.updateDraft}
          onBack={() => workflow.goToStep('bdinfo')}
          onNext={() => workflow.goToStep('review')}
        />
      ) : null}

      {workflow.step === 'review' && workflow.selectedSource && workflow.parsedBDInfo && workflow.draft ? (
        <ReviewPage
          locale={workflow.locale}
          source={workflow.selectedSource}
          bdinfo={workflow.parsedBDInfo}
          draft={workflow.draft}
          outputFilename={workflow.outputFilename || workflow.filenamePreview}
          outputPath={workflow.outputPath}
          submitting={workflow.submittingJob}
          startDisabled={workflow.currentJob?.status === 'running'}
          submitError={workflow.submitError}
          currentJob={workflow.currentJob}
          currentLog={workflow.currentJobLog}
          onBack={() => workflow.goToStep('editor')}
          onStartNextRemux={workflow.handleStartNextRemux}
          onSubmit={workflow.handleSubmitJob}
        />
      ) : null}
    </Layout>
  );
}

export default App;
