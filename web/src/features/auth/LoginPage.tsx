import { FormEvent, useState } from 'react';
import { Button } from '../../components/Button';
import { getMessages, type Locale } from '../../i18n';

type LoginPageProps = {
  locale?: Locale;
  onSuccess: (password: string) => Promise<void> | void;
  error?: string | null;
};

export function LoginPage({ locale = 'zh', onSuccess, error: externalError }: LoginPageProps) {
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const text = getMessages(locale);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!password.trim()) {
      setError(text.login.passwordRequired);
      return;
    }

    setError(null);
    await onSuccess(password.trim());
  };

  return (
    <section className="login-screen">
      <div className="login-card">
        <div className="login-brand-mark">MM</div>
        <div className="login-panel-intro">
          <h2>{text.login.title}</h2>
        </div>
        <form onSubmit={handleSubmit} className="login-form login-card-form">
          <div className="login-form-field">
            <label htmlFor="password">{text.login.passwordLabel}</label>
            <input
              id="password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
              placeholder={text.login.passwordPlaceholder}
            />
          </div>
          {error ? <p className="error-text">{error}</p> : null}
          {externalError ? <p className="error-text">{externalError}</p> : null}
          <Button type="submit" className="login-submit-button">
            {text.login.continueButton}
          </Button>
        </form>
      </div>
    </section>
  );
}
