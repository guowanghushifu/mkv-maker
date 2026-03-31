import type { ElementType, ReactNode } from 'react';

type SummaryCardProps = {
  as?: ElementType;
  className?: string;
  labelClassName?: string;
  valueClassName?: string;
  label: ReactNode;
  value: ReactNode;
  children?: ReactNode;
};

export function SummaryCard({
  as: Component = 'article',
  className = '',
  labelClassName = 'summary-label',
  valueClassName = 'summary-value',
  label,
  value,
  children,
}: SummaryCardProps) {
  return (
    <Component className={className}>
      <span className={labelClassName}>{label}</span>
      <strong className={valueClassName}>{value}</strong>
      {children}
    </Component>
  );
}
