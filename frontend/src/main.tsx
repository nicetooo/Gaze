import React from 'react'
import {createRoot} from 'react-dom/client'
import { ConfigProvider } from 'antd'
import './style.css'
import './i18n'
import App from './App'
import { ThemeProvider } from './ThemeContext'

const container = document.getElementById('root')

// Global log capture for Feedback
const originalLog = console.log;
const originalWarn = console.warn;
const originalError = console.error;

(window as any).runtimeLogs = [];

const captureLog = (type: string, ...args: any[]) => {
  const msg = `[${new Date().toLocaleTimeString()}] [${type}] ${args.map(a => {
    try {
      return typeof a === 'object' ? JSON.stringify(a) : String(a);
    } catch (e) {
      return String(a);
    }
  }).join(' ')}`;
  (window as any).runtimeLogs.push(msg);
  if ((window as any).runtimeLogs.length > 1000) (window as any).runtimeLogs.shift();
};

console.log = (...args) => { originalLog(...args); captureLog('LOG', ...args); };
console.warn = (...args) => { originalWarn(...args); captureLog('WARN', ...args); };
console.error = (...args) => { originalError(...args); captureLog('ERROR', ...args); };

const root = createRoot(container!)

root.render(
    <React.StrictMode>
        <ThemeProvider>
            <App/>
        </ThemeProvider>
    </React.StrictMode>
)
