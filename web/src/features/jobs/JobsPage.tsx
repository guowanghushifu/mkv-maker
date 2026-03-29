import type { Job } from '../../api/types';
import { StatusBadge } from '../../components/StatusBadge';

type JobsPageProps = {
  jobs: Job[];
  onStartNew: () => void;
  onRefresh: () => Promise<void> | void;
};

export function JobsPage({ jobs, onStartNew, onRefresh }: JobsPageProps) {
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
            <li key={job.id}>
              <div>
                <strong>{job.outputName}</strong>
                <small>{new Date(job.createdAt).toLocaleString()}</small>
              </div>
              <div>
                <small>Playlist {job.playlistName}</small>
                <StatusBadge status={job.status} />
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

