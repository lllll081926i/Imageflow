import React from 'react';
import Icon from './Icon';

export const WindowControls: React.FC = () => {
    const quit = () => {
        window.runtime?.Quit?.();
    };

    const minimize = () => {
        window.runtime?.WindowMinimise?.();
    };

    const maximize = () => {
        window.runtime?.WindowToggleMaximise?.();
    };

    return (
        <div className="flex items-center gap-1 h-full px-2" style={{ ['--wails-draggable' as any]: 'no-drag' }}>
            <button 
                onClick={minimize}
                className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-gray-200 dark:hover:bg-white/10 text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white transition-colors"
                title="最小化"
            >
                <Icon name="Minimize" size={14} />
            </button>
            <button 
                onClick={maximize}
                className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-gray-200 dark:hover:bg-white/10 text-gray-500 hover:text-gray-900 dark:text-gray-400 dark:hover:text-white transition-colors"
                title="最大化"
            >
                <Icon name="Maximize" size={14} />
            </button>
            <button 
                onClick={quit}
                className="w-8 h-8 flex items-center justify-center rounded-lg hover:bg-red-500 text-gray-500 hover:text-white dark:text-gray-400 dark:hover:text-white transition-colors group"
                title="关闭"
            >
                <Icon name="Close" size={14} />
            </button>
        </div>
    );
};
