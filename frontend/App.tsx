import React, { useState, useEffect } from 'react';
import Sidebar from './components/Sidebar';
import Dashboard from './components/Dashboard';
import DetailView from './components/DetailView';
import { WindowControls } from './components/WindowControls';
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
        <div className={`w-full h-screen overflow-hidden flex flex-col bg-[#F5F5F7] dark:bg-[#1E1E1E] text-gray-900 transition-colors duration-300`}>
            {/* Custom Title Bar */}
            <div 
                className="h-11 flex items-center justify-between px-4 bg-[#F5F5F7] dark:bg-[#1E1E1E] select-none shrink-0 z-50 relative"
                style={{ ['--wails-draggable' as any]: 'drag' }}
            >
                {/* Logo & Title */}
                <div className="flex items-center gap-3 opacity-90 hover:opacity-100 transition-opacity">
                    <div className="w-6 h-6 flex items-center justify-center">
                        <Icon name="AppLogo" size={22} />
                    </div>
                    <span className="text-sm font-semibold tracking-wide bg-clip-text text-transparent bg-gradient-to-r from-[#007AFF] via-[#5856D6] to-[#AF52DE]">ImageFlow</span>
                </div>

                <WindowControls />
            </div>

            <div className="flex-1 flex overflow-hidden">
                <Sidebar active={activeView} setActive={handleNavigate} theme={theme} toggleTheme={toggleTheme} />
                <main className="flex-1 flex flex-col h-full relative z-10">
                    <div className="flex-1 overflow-y-auto p-4 md:p-8 overflow-x-hidden no-scrollbar">
                        <div className="max-w-7xl mx-auto h-full">
                            <div key={activeView} className="h-full animate-fade-scale">
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
        </div>
    );
};

export default App;
