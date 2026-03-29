import { useState } from 'react';
import type { Job } from '../../api/types';
import { StatusBadge } from '../../components/StatusBadge';

type JobsPageProps = {
  jobs: Job[];
  onStartNew: () => void;
  onRefresh: () => Promise<void> | void;
  onLoadLog: (jobId: string) => Promise<string>;
};

export function JobsPage({ jobs, onStartNew, onRefresh, onLoadLog }: JobsPageProps) {
  const [expandedJobId, setExpandedJobId] = useState<string | null>(null);
  const [logsByJobId, setLogsByJobId] = useState<Record<string, string>>({});
  const [loadingJobId, setLoadingJobId] = useState<string | null>(null);

  const toggleLog = async (jobId: string) => {
    if (expandedJobId === jobId) {
      setExpandedJobId(null);
      return;
    }

    if (!logsByJobId[jobId]) {
      setLoadingJobId(jobId);
      const logText = await onLoadLog(jobId);
      setLogsByJobId((current) => ({ ...current, [jobId]: logText }));
      setLoadingJobId(null);
    }
    setExpandedJobId(jobId);
  };

  return (
    <section className="panel">
      <h2>Jobs</h2>
      <div className="row">
        <button type="button" onClick={() => void onRefresh()}>
          Refresh
        </button>
        <button type="button" onClick={onStartNew}>
          New Workflow
        </button>
      </div>

      {jobs.length === 0 ? (
        <p className="muted-text">No jobs yet. Queue a remux job from the review step.</p>
      ) : (
        <ul className="job-list">
          {jobs.map((job) => (
            <li key={job.id} className="job-list-item">
              <div className="job-row">
                <div>
                  <strong>{job.outputName}</strong>
                  <small>{new Date(job.createdAt).toLocaleString()}</small>
                  <small>{job.outputPath || 'No output path available'}</small>
                </div>
                <div className="job-side">
                  <small>Playlist {job.playlistName}</small>
                  <StatusBadge status={job.status} />
                  <button type="button" onClick={() => void toggleLog(job.id)}>
                    {expandedJobId === job.id ? 'Hide Log' : 'View Log'}
                  </button>
                </div>
              </div>
              {expandedJobId === job.id ? (
                <pre className="job-log" aria-label={`Job log ${job.id}`}>
                  {loadingJobId === job.id ? 'Loading log...' : logsByJobId[job.id]}
                </pre>
              ) : null}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

