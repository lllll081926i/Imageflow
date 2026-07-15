import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './index.css';
import { installDesktopRuntime } from './runtime/desktopRuntime';
import ErrorBoundary from './components/ErrorBoundary';

installDesktopRuntime();

const rootElement = document.getElementById('root');
if (!rootElement) {
  throw new Error("Could not find root element to mount to");
}

const root = ReactDOM.createRoot(rootElement);
root.render(
  <React.StrictMode>
    <ErrorBoundary title="应用渲染出错">
      <App />
    </ErrorBoundary>
  </React.StrictMode>
);
