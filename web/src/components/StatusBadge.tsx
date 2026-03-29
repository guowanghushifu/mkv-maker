import type { JobStatus } from '../api/types';

type StatusBadgeProps = {
  status: JobStatus;
};

const statusLabels: Record<JobStatus, string> = {
  running: 'Running',
  succeeded: 'Succeeded',
  failed: 'Failed',
};

export function StatusBadge({ status }: StatusBadgeProps) {
  return <span className={`status-badge status-${status}`}>{statusLabels[status]}</span>;
}
