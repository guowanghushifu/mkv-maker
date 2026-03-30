import type { JobStatus } from '../api/types';
import { getMessages, type Locale } from '../i18n';

type StatusBadgeProps = {
  status: JobStatus;
  locale?: Locale;
};

export function StatusBadge({ status, locale = 'zh' }: StatusBadgeProps) {
  const text = getMessages(locale);
  return <span className={`status-badge status-${status}`}>{text.status[status]}</span>;
}
