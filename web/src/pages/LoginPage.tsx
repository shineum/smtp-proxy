import { useState, type FormEvent } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  LoginPage as PFLoginPage,
  LoginForm,
  ListVariant,
} from '@patternfly/react-core';
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

  const loginForm = (
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
  );

  return (
    <PFLoginPage
      brandImgSrc=""
      brandImgAlt=""
      loginTitle="SMTP Proxy Admin"
      loginSubtitle="Sign in to manage your email infrastructure"
      textContent=""
      footerListVariants={ListVariant.inline}
    >
      {loginForm}
    </PFLoginPage>
  );
}
