import { FormEvent, useState } from 'react';

type LoginPageProps = {
  onSuccess: (password: string) => Promise<void> | void;
};

export function LoginPage({ onSuccess }: LoginPageProps) {
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!password.trim()) {
      setError('Password is required.');
      return;
    }

    setError(null);
    await onSuccess(password.trim());
  };

  return (
    <section className="panel">
      <h2>Login</h2>
      <p>Single-user local access.</p>
      <form onSubmit={handleSubmit} className="stack">
        <label htmlFor="password">Password</label>
        <input
          id="password"
          type="password"
          value={password}
          onChange={(event) => setPassword(event.target.value)}
          placeholder="Enter password"
        />
        {error ? <p className="error-text">{error}</p> : null}
        <button type="submit">Continue</button>
      </form>
    </section>
  );
}

