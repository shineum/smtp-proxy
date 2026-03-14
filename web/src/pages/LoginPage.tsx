import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  LoginForm,
} from '@patternfly/react-core';
import { EnvelopeIcon, ShieldAltIcon } from '@patternfly/react-icons';
import { useAuth } from '../context/AuthContext';

export default function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [isLoading, setIsLoading] = useState(false);

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault();
    setError('');
    setIsLoading(true);
    try {
      await login(email, password);
      navigate('/');
    } catch (err: unknown) {
      const msg = (err as { response?: { data?: { error?: string } } })?.response?.data?.error || 'Login failed';
      setError(msg);
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <div className="login-page-wrapper">
      <div className="login-card">
        <div className="login-brand">
          <div className="login-brand-icons">
            <ShieldAltIcon className="login-brand-icon shield" />
            <EnvelopeIcon className="login-brand-icon envelope" />
          </div>
          <h1 className="login-brand-title">SMTP Proxy</h1>
          <p className="login-brand-subtitle">Admin Console</p>
        </div>

        <div className="login-form-wrapper">
          <LoginForm
            showHelperText={!!error}
            helperText={error}
            helperTextIcon={undefined}
            usernameLabel="Email"
            usernameValue={email}
            onChangeUsername={(_e, v) => setEmail(v)}
            passwordLabel="Password"
            passwordValue={password}
            onChangePassword={(_e, v) => setPassword(v)}
            onLoginButtonClick={handleSubmit}
            loginButtonLabel={isLoading ? 'Signing in...' : 'Sign in'}
            isLoginButtonDisabled={isLoading}
          />
        </div>

        <p className="login-footer-text">
          Secure email infrastructure management
        </p>
      </div>
    </div>
  );
}
