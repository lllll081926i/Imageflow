import React, { useState, useEffect } from 'react';
import Sidebar from './components/Sidebar';
import Dashboard from './components/Dashboard';
import { WindowControls } from './components/WindowControls';
import Icon from './components/Icon';
import DetailView from './components/DetailView';
import SettingsView from './components/SettingsView';
import { ViewState, Theme } from './types';
import { FEATURES } from './constants';

const FEATURE_IDS = FEATURES.map((feature) => feature.id);

const App: React.FC = () => {
    const [theme, setTheme] = useState<Theme>('light');
    const [activeView, setActiveView] = useState<ViewState>('dashboard');
    const [direction, setDirection] = useState<'left' | 'right'>('right');
    const [loadedFeatureViews, setLoadedFeatureViews] = useState<ViewState[]>([]);
    const isSecondaryView = activeView !== 'dashboard';
    const isFeatureView = FEATURE_IDS.includes(activeView);
    const visibleFeatureViews = isFeatureView && !loadedFeatureViews.includes(activeView)
        ? [...loadedFeatureViews, activeView]
        : loadedFeatureViews;

    useEffect(() => {
        // Prevent default drag behaviors to stop opening files in browser
        const preventDefault = (e: DragEvent) => {
            e.preventDefault();
            e.stopPropagation();
        };

        const preventCtrlZoom = (e: WheelEvent) => {
            if (e.ctrlKey) {
                e.preventDefault();
            }
        };

        const preventCtrlKeyZoom = (e: KeyboardEvent) => {
            if (!e.ctrlKey) return;
            const key = e.key;
            if (key === '+' || key === '-' || key === '=' || key === '0') {
                e.preventDefault();
            }
            if (key === 'NumpadAdd' || key === 'NumpadSubtract') {
                e.preventDefault();
            }
        };
        
        window.addEventListener('dragover', preventDefault);
        window.addEventListener('drop', preventDefault);
        window.addEventListener('wheel', preventCtrlZoom, { passive: false });
        window.addEventListener('keydown', preventCtrlKeyZoom);

        // Check system preference on load
        if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
            setTheme('dark');
            document.documentElement.classList.add('dark');
        }

        return () => {
            window.removeEventListener('dragover', preventDefault);
            window.removeEventListener('drop', preventDefault);
            window.removeEventListener('wheel', preventCtrlZoom);
            window.removeEventListener('keydown', preventCtrlKeyZoom);
        };
    }, []);

    useEffect(() => {
        if (!isFeatureView) return;
        setLoadedFeatureViews((prev) => (prev.includes(activeView) ? prev : [...prev, activeView]));
    }, [activeView, isFeatureView]);

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
                <div 
                    className="flex items-center gap-3 opacity-90 hover:opacity-100 transition-opacity cursor-pointer group"
                    onClick={() => handleNavigate('dashboard')}
                    style={{ ['--wails-draggable' as any]: 'no-drag' }}
                >
                    <div className="w-6 h-6 flex items-center justify-center transition-transform group-hover:scale-105">
                        <Icon name="AppLogo" size={22} />
                    </div>
                    <span className="text-sm font-semibold tracking-wide bg-clip-text text-transparent bg-gradient-to-r from-[#007AFF] via-[#5856D6] to-[#AF52DE]">ImageFlow</span>
                </div>

                {/* Window Controls */}
                <div className="flex items-center gap-2">
                    <button 
                        onClick={toggleTheme}
                        className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-gray-200 dark:hover:bg-white/10 text-gray-500 dark:text-gray-400 transition-all active:scale-90 cursor-pointer z-50"
                        style={{ ['--wails-draggable' as any]: 'no-drag' }}
                        title={theme === 'light' ? '切换到深色模式' : '切换到浅色模式'}
                    >
                        <Icon name={theme === 'light' ? 'Moon' : 'Sun'} size={16} />
                    </button>
                    <WindowControls />
                </div>
            </div>

            <div className="flex-1 flex overflow-hidden">
                <Sidebar active={activeView} setActive={handleNavigate} />
                <main className="flex-1 flex flex-col h-full relative z-10 overflow-hidden">
                    <div className={`flex-1 overflow-hidden ${isSecondaryView ? 'px-4 py-3 md:px-6 md:py-4' : 'p-4 md:p-6'}`}>
                        <div className="max-w-full mx-auto h-full">
                            <div className="h-full animate-fade-scale flex flex-col">
                                <div className={`${activeView === 'dashboard' ? 'block' : 'hidden'} h-full`}>
                                    <Dashboard onSelect={handleNavigate} />
                                </div>
                                <div className={`${activeView === 'settings' ? 'block' : 'hidden'} h-full`}>
                                    <SettingsView />
                                </div>
                                {visibleFeatureViews.map((viewId) => (
                                    <div key={viewId} className={`${activeView === viewId ? 'block' : 'hidden'} h-full`}>
                                        <DetailView
                                            id={viewId}
                                            isActive={activeView === viewId}
                                            onBack={() => handleNavigate('dashboard')}
                                        />
                                    </div>
                                ))}
                            </div>
                        </div>
                    </div>
                </main>
            </div>
        </div>
    );
};

export default App;
