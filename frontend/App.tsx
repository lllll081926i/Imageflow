import React, { useCallback, useEffect, useRef, useState } from 'react';
import Sidebar from './components/Sidebar';
import Dashboard from './components/Dashboard';
import { WindowControls } from './components/WindowControls';
import Icon from './components/Icon';
import DetailView from './components/DetailView';
import SettingsView from './components/SettingsView';
import { ViewState, Theme, FeatureId } from './types';
import { FEATURES } from './constants';

const FEATURE_IDS = FEATURES.map((feature) => feature.id) as FeatureId[];

const isFeatureId = (view: ViewState): view is FeatureId => FEATURE_IDS.includes(view as FeatureId);

type TaskFailureNotification = {
    id: string;
    taskName: string;
    imageName: string;
    reason: string;
    createdAt: number;
};

const App: React.FC = () => {
    const [theme, setTheme] = useState<Theme>('light');
    const [activeView, setActiveView] = useState<ViewState>('dashboard');
    const [direction, setDirection] = useState<'left' | 'right'>('right');
    const [loadedFeatureViews, setLoadedFeatureViews] = useState<FeatureId[]>([]);
    const [failureNotifications, setFailureNotifications] = useState<TaskFailureNotification[]>([]);
    const [hasUnreadNotifications, setHasUnreadNotifications] = useState(false);
    const [isNotificationOpen, setIsNotificationOpen] = useState(false);
    const closeNotificationTimerRef = useRef<number | null>(null);
    const notificationContainerRef = useRef<HTMLDivElement | null>(null);
    const isSecondaryView = activeView !== 'dashboard';
    const isFeatureView = isFeatureId(activeView);
    const visibleFeatureViews = isFeatureView && !loadedFeatureViews.includes(activeView)
        ? [...loadedFeatureViews, activeView]
        : loadedFeatureViews;

    useEffect(() => {
        // Prevent default drag behaviors to stop opening files in browser
        const preventDefault = (e: DragEvent) => {
            e.preventDefault();
            e.stopPropagation();
        };

        window.addEventListener('dragover', preventDefault);
        window.addEventListener('drop', preventDefault);

        // Check system preference on load
        if (window.matchMedia('(prefers-color-scheme: dark)').matches) {
            setTheme('dark');
            document.documentElement.classList.add('dark');
        }

        return () => {
            if (closeNotificationTimerRef.current) {
                window.clearTimeout(closeNotificationTimerRef.current);
                closeNotificationTimerRef.current = null;
            }
            window.removeEventListener('dragover', preventDefault);
            window.removeEventListener('drop', preventDefault);
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

    const handleNavigate = useCallback((view: ViewState) => {
        if (view === 'dashboard') {
            setDirection('left');
        } else {
            setDirection('right');
        }
        setActiveView(view);
    }, []);

    const handlePreload = useCallback((view: ViewState) => {
        if (!isFeatureId(view)) return;
        setLoadedFeatureViews((prev) => (prev.includes(view) ? prev : [...prev, view]));
    }, []);

    const handleBack = useCallback(() => handleNavigate('dashboard'), [handleNavigate]);

    const handleTaskFailure = useCallback((payload: { taskName: string; imageName: string; reason: string }) => {
        const id = `${Date.now()}-${Math.random().toString(16).slice(2)}`;
        const next: TaskFailureNotification = {
            id,
            taskName: payload.taskName,
            imageName: payload.imageName,
            reason: payload.reason,
            createdAt: Date.now(),
        };
        setFailureNotifications((prev) => [next, ...prev].slice(0, 80));
        setHasUnreadNotifications(true);
    }, []);

    const handleNotificationMouseEnter = useCallback(() => {
        if (closeNotificationTimerRef.current) {
            window.clearTimeout(closeNotificationTimerRef.current);
            closeNotificationTimerRef.current = null;
        }
        setIsNotificationOpen(true);
        if (hasUnreadNotifications) {
            setHasUnreadNotifications(false);
        }
    }, [hasUnreadNotifications]);

    const handleNotificationMouseLeave = useCallback(() => {
        if (closeNotificationTimerRef.current) {
            window.clearTimeout(closeNotificationTimerRef.current);
        }
        closeNotificationTimerRef.current = window.setTimeout(() => {
            setIsNotificationOpen(false);
        }, 120);
    }, []);

    const handleNotificationButtonClick = useCallback(() => {
        if (closeNotificationTimerRef.current) {
            window.clearTimeout(closeNotificationTimerRef.current);
            closeNotificationTimerRef.current = null;
        }
        setIsNotificationOpen((prev) => !prev);
        if (hasUnreadNotifications) {
            setHasUnreadNotifications(false);
        }
    }, [hasUnreadNotifications]);

    const handleDismissNotification = useCallback((id: string) => {
        setFailureNotifications((prev) => prev.filter((item) => item.id !== id));
    }, []);

    const handleClearNotifications = useCallback(() => {
        setFailureNotifications([]);
        setHasUnreadNotifications(false);
    }, []);

    const formatNotificationTime = useCallback((timestamp: number) => {
        const date = new Date(timestamp);
        const now = Date.now();
        const diffMs = Math.max(0, now - date.getTime());
        const diffMin = Math.floor(diffMs / 60000);
        if (diffMin < 1) return '刚刚';
        if (diffMin < 60) return `${diffMin} 分钟前`;
        const diffHour = Math.floor(diffMin / 60);
        if (diffHour < 24) return `${diffHour} 小时前`;
        return date.toLocaleString();
    }, []);

    useEffect(() => {
        if (!isNotificationOpen) return;
        const handlePointerDown = (event: MouseEvent) => {
            const target = event.target;
            if (!(target instanceof Node)) return;
            if (notificationContainerRef.current?.contains(target)) return;
            setIsNotificationOpen(false);
        };
        const handleEscape = (event: KeyboardEvent) => {
            if (event.key !== 'Escape') return;
            setIsNotificationOpen(false);
        };
        window.addEventListener('mousedown', handlePointerDown);
        window.addEventListener('keydown', handleEscape);
        return () => {
            window.removeEventListener('mousedown', handlePointerDown);
            window.removeEventListener('keydown', handleEscape);
        };
    }, [isNotificationOpen]);

    return (
        <div className={`w-full h-screen overflow-hidden flex flex-col bg-[#F5F5F7] dark:bg-[#1E1E1E] text-gray-900 transition-colors duration-300`}>
            {/* Custom Title Bar */}
            <div 
                className="h-11 flex items-center justify-between px-4 bg-[#F5F5F7] dark:bg-[#1E1E1E] select-none shrink-0 z-50 relative"
                style={{ ['--wails-draggable' as any]: 'drag' }}
            >
                {/* Logo & Title */}
                <button
                    type="button"
                    className="flex items-center gap-3 opacity-90 hover:opacity-100 transition-opacity cursor-pointer group"
                    onClick={() => handleNavigate('dashboard')}
                    style={{ ['--wails-draggable' as any]: 'no-drag' }}
                    title="返回主页"
                >
                    <div className="w-6 h-6 flex items-center justify-center transition-transform group-hover:scale-105">
                        <Icon name="AppLogo" size={22} />
                    </div>
                    <span className="text-sm font-semibold tracking-wide bg-clip-text text-transparent bg-gradient-to-r from-[#007AFF] via-[#5856D6] to-[#AF52DE]">ImageFlow</span>
                </button>

                {/* Window Controls */}
                <div className="flex items-center gap-2">
                    <div
                        ref={notificationContainerRef}
                        className="relative"
                        style={{ ['--wails-draggable' as any]: 'no-drag' }}
                        onMouseEnter={handleNotificationMouseEnter}
                        onMouseLeave={handleNotificationMouseLeave}
                        onFocusCapture={handleNotificationMouseEnter}
                        onBlurCapture={(e) => {
                            const next = e.relatedTarget;
                            if (next instanceof Node && notificationContainerRef.current?.contains(next)) {
                                return;
                            }
                            handleNotificationMouseLeave();
                        }}
                    >
                        <button
                            className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-gray-200 dark:hover:bg-white/10 text-gray-500 dark:text-gray-400 transition-all active:scale-90 cursor-pointer z-50 relative"
                            title="通知"
                            aria-expanded={isNotificationOpen}
                            aria-haspopup="dialog"
                            aria-controls="notification-panel"
                            onClick={handleNotificationButtonClick}
                        >
                            <Icon name="Bell" size={16} />
                            {hasUnreadNotifications && failureNotifications.length > 0 && (
                                <span className="absolute top-1.5 right-1.5 w-2 h-2 rounded-full bg-red-500 shadow-[0_0_0_2px_rgba(245,245,247,1)] dark:shadow-[0_0_0_2px_rgba(30,30,30,1)]" />
                            )}
                        </button>
                        <div
                            id="notification-panel"
                            role="dialog"
                            aria-label="任务通知"
                            className={`absolute right-0 top-10 w-[320px] max-h-[360px] overflow-hidden rounded-xl border border-gray-200/80 dark:border-white/10 bg-white/95 dark:bg-[#232326]/95 shadow-xl backdrop-blur-sm z-[120] origin-top-right transition-all duration-150 ease-out ${isNotificationOpen ? 'opacity-100 translate-y-0 scale-100 pointer-events-auto' : 'opacity-0 -translate-y-1 scale-95 pointer-events-none'}`}
                        >
                            <div className="px-3 py-2 border-b border-gray-100 dark:border-white/10 flex items-center justify-between gap-2 text-xs font-semibold text-gray-600 dark:text-gray-300">
                                <span>通知 {failureNotifications.length > 0 ? `(${failureNotifications.length})` : ''}</span>
                                <button
                                    type="button"
                                    disabled={failureNotifications.length === 0}
                                    onClick={handleClearNotifications}
                                    className="text-[11px] text-gray-500 hover:text-red-500 disabled:text-gray-300 disabled:cursor-not-allowed transition-colors"
                                >
                                    清空
                                </button>
                            </div>
                            {failureNotifications.length === 0 ? (
                                <div className="px-3 py-6 text-xs text-gray-500 dark:text-gray-400 text-center">暂无通知</div>
                            ) : (
                                <div className="max-h-[316px] overflow-y-auto no-scrollbar p-2 space-y-2">
                                    {failureNotifications.map((item) => (
                                        <div key={item.id} className="rounded-lg border border-red-100 dark:border-red-500/20 bg-red-50/70 dark:bg-red-500/10 px-2.5 py-2">
                                            <div className="flex items-start justify-between gap-2">
                                                <div className="text-xs font-medium text-red-700 dark:text-red-300 truncate">{item.taskName} · {item.imageName}</div>
                                                <button
                                                    type="button"
                                                    onClick={() => handleDismissNotification(item.id)}
                                                    className="shrink-0 text-red-400 hover:text-red-600 dark:hover:text-red-200 transition-colors"
                                                    title="移除通知"
                                                    aria-label="移除通知"
                                                >
                                                    <Icon name="X" size={12} />
                                                </button>
                                            </div>
                                            <div className="text-[11px] text-red-600/90 dark:text-red-200/85 mt-0.5 leading-4 break-all">{item.reason}</div>
                                            <div className="text-[10px] text-red-500/80 dark:text-red-200/70 mt-1">{formatNotificationTime(item.createdAt)}</div>
                                        </div>
                                    ))}
                                </div>
                            )}
                        </div>
                    </div>
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
                <Sidebar active={activeView} setActive={handleNavigate} onPreload={handlePreload} />
                <main className="flex-1 flex flex-col h-full relative z-10 overflow-hidden">
                    <div className={`flex-1 overflow-hidden ${isSecondaryView ? 'px-4 py-3 md:px-6 md:py-4' : 'p-4 md:p-6'}`}>
                        <div className="max-w-full mx-auto h-full">
                            <div className="h-full animate-fade-scale flex flex-col">
                                <div className={`${activeView === 'dashboard' ? 'block' : 'hidden'} h-full`}>
                                    <Dashboard onSelect={handleNavigate} onPreload={handlePreload} />
                                </div>
                                <div className={`${activeView === 'settings' ? 'block' : 'hidden'} h-full`}>
                                    <SettingsView />
                                </div>
                                {visibleFeatureViews.map((viewId) => (
                                    <div key={viewId} className={`${activeView === viewId ? 'block' : 'hidden'} h-full`}>
                                        <DetailView
                                            id={viewId}
                                            isActive={activeView === viewId}
                                            onBack={handleBack}
                                            onTaskFailure={handleTaskFailure}
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
