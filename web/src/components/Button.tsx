import type { ButtonHTMLAttributes, ReactNode } from 'react';

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  variant?: 'primary' | 'subtle';
};

export function Button({ children, className = '', type = 'button', variant = 'primary', ...props }: ButtonProps) {
  const classes = ['ui-button', variant === 'subtle' ? 'ui-button-subtle' : 'ui-button-primary', className]
    .filter(Boolean)
    .join(' ');

  return (
    <button type={type} className={classes} {...props}>
      {children}
    </button>
  );
}
