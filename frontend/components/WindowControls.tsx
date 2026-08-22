import React from 'react';
import Icon from './Icon';

export const WindowControls: React.FC = () => {
    const quit = () => {
        const bindings = (window as { pywebview?: { api?: { CloseRequest?: () => void } } }).pywebview?.api;
        if (bindings?.CloseRequest) {
            void bindings.CloseRequest();
            return;
        }
        window.runtime?.Quit?.();
    };

    const minimize = () => {
        window.runtime?.WindowMinimise?.();
    };

    const maximize = () => {
        window.runtime?.WindowToggleMaximise?.();
    };

    return (
        <div className="flex items-center gap-1 h-full px-2">
            <button
                onClick={minimize}
                title="最小化"
                aria-label="最小化"
                className="spring-tap-sm w-9 h-7 flex items-center justify-center rounded-lg hover:bg-gray-200 dark:hover:bg-white/10 text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white transition-colors"
            >
                <Icon name="Minimize" size={14} />
            </button>
            <button
                onClick={maximize}
                title="最大化 / 还原"
                aria-label="最大化或还原"
                className="spring-tap-sm w-9 h-7 flex items-center justify-center rounded-lg hover:bg-gray-200 dark:hover:bg-white/10 text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white transition-colors"
            >
                <Icon name="Maximize" size={13} />
            </button>
            <button
                onClick={quit}
                title="关闭"
                aria-label="关闭"
                className="spring-tap-sm w-9 h-7 flex items-center justify-center rounded-lg hover:bg-red-500 text-gray-500 hover:text-white dark:text-gray-400 dark:hover:text-white transition-colors"
            >
                <Icon name="Close" size={14} />
            </button>
        </div>
    );
};
