type SwitchProps = {
  checked: boolean;
  disabled?: boolean;
  className?: string;
  'aria-label': string;
  onChange?: () => void;
};

export function Switch({
  checked,
  disabled = false,
  className = '',
  'aria-label': ariaLabel,
  onChange,
}: SwitchProps) {
  const classes = ['ui-switch', checked ? 'is-on' : 'is-off', className].filter(Boolean).join(' ');

  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      disabled={disabled}
      className={classes}
      onClick={() => onChange?.()}
    >
      <span className="ui-switch-track" aria-hidden="true">
        <span className="ui-switch-thumb" />
      </span>
    </button>
  );
}
