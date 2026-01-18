import React, { useState, useEffect } from 'react';
import Sidebar from './components/Sidebar';
import Dashboard from './components/Dashboard';
import DetailView from './components/DetailView';
import Icon from './components/Icon';
import { ViewState, Theme } from './types';

const App: React.FC = () => {
    const [theme, setTheme] = useState<Theme>('light');
    const [activeView, setActiveView] = useState<ViewState>('dashboard');
    const [direction, setDirection] = useState<'left' | 'right'>('right');

    useEffect(() => {
        // Check system preference on load
        if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
            setTheme('dark');
            document.documentElement.classList.add('dark');
        }
    }, []);

    const toggleTheme = () => {
        if (theme === 'light') {
            setTheme('dark');
            document.documentElement.classList.add('dark');
        } else {
            setTheme('light');
            document.documentElement.classList.remove('dark');
        }
    };

    const handleNavigate = (view: ViewState) => {
        if (view === 'dashboard') {
            setDirection('left');
        } else {
            setDirection('right');
        }
        setActiveView(view);
    };

    return (
        <div className={`w-full h-screen overflow-hidden flex bg-[#F5F5F7] dark:bg-[#1E1E1E] text-gray-900 transition-colors duration-300`}>
            <Sidebar active={activeView} setActive={handleNavigate} />
            <main className="flex-1 flex flex-col h-full relative z-10">
                <header className="h-16 flex items-center justify-between px-8 shrink-0">
                    <div className="flex items-center gap-2 text-sm text-gray-400 dark:text-gray-500">
                        <Icon name="Command" size={14} /> <span className="font-medium">ImageFlow v0.1 Alpha</span>
                    </div>
                    <button 
                        onClick={toggleTheme} 
                        className="w-10 h-10 rounded-full flex items-center justify-center hover:bg-gray-200 dark:hover:bg-white/10 text-gray-600 dark:text-gray-300 transition-all"
                    >
                        <Icon name={theme === 'light' ? 'Moon' : 'Sun'} size={20} />
                    </button>
                </header>
                <div className="flex-1 overflow-y-auto p-4 md:p-8 pt-0 overflow-x-hidden">
                    <div className="max-w-7xl mx-auto h-full">
                        <div key={activeView} className={`h-full ${direction === 'right' ? 'animate-enter-right' : 'animate-enter-left'}`}>
                            {activeView === 'dashboard' ? (
                                <Dashboard onSelect={handleNavigate} />
                            ) : (
                                <DetailView id={activeView} onBack={() => handleNavigate('dashboard')} />
                            )}
                        </div>
                    </div>
                </div>
            </main>
        </div>
    );
};

export default App;