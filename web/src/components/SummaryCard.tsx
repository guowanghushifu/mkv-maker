import type { ComponentPropsWithoutRef, ElementType, ReactNode } from 'react';

type SummaryCardProps = {
  as?: ElementType;
  className?: string;
  labelClassName?: string;
  valueClassName?: string;
  valueProps?: ComponentPropsWithoutRef<'strong'>;
  label: ReactNode;
  value: ReactNode;
  children?: ReactNode;
};

export function SummaryCard({
  as: Component = 'article',
  className = '',
  labelClassName = 'summary-label',
  valueClassName = 'summary-value',
  valueProps,
  label,
  value,
  children,
}: SummaryCardProps) {
  return (
    <Component className={className}>
      <span className={labelClassName}>{label}</span>
      <strong className={valueClassName} {...valueProps}>{value}</strong>
      {children}
    </Component>
  );
}
