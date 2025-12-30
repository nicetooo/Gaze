import React, { createContext, useContext, useEffect, useState } from 'react';
import { ConfigProvider, theme as antTheme } from 'antd';
import { getTheme } from './theme';

type ThemeMode = 'light' | 'dark' | 'system';

interface ThemeContextType {
  mode: ThemeMode;
  setMode: (mode: ThemeMode) => void;
  isDark: boolean;
}

const ThemeContext = createContext<ThemeContextType>({
  mode: 'system',
  setMode: () => {},
  isDark: false,
});

export const useTheme = () => useContext(ThemeContext);

export const ThemeProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [mode, setMode] = useState<ThemeMode>(() => {
    return (localStorage.getItem('theme-mode') as ThemeMode) || 'system';
  });
  
  const [isDark, setIsDark] = useState(false);

  useEffect(() => {
    localStorage.setItem('theme-mode', mode);

    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
    
    const handleChange = () => {
      if (mode === 'system') {
        setIsDark(mediaQuery.matches);
      }
    };

    if (mode === 'system') {
      setIsDark(mediaQuery.matches);
      mediaQuery.addEventListener('change', handleChange);
    } else {
      setIsDark(mode === 'dark');
    }

    return () => mediaQuery.removeEventListener('change', handleChange);
  }, [mode]);

  const themeConfig = getTheme(isDark ? 'dark' : 'light');

  return (
    <ThemeContext.Provider value={{ mode, setMode, isDark }}>
      <ConfigProvider theme={themeConfig}>
        {children}
      </ConfigProvider>
    </ThemeContext.Provider>
  );
};
