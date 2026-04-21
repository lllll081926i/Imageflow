import React, { useLayoutEffect, useMemo, useRef, useState } from 'react';
import Icon from './Icon';
import { FEATURES } from '../constants';
import { ViewState } from '../types';

interface SidebarProps {
    active: ViewState;
    setActive: (view: ViewState) => void;
}

const Sidebar: React.FC<SidebarProps> = ({ active, setActive }) => {
    const [collapsed, setCollapsed] = useState(false);
    const listRef = useRef<HTMLDivElement>(null);
    const itemRefs = useRef<Record<string, HTMLButtonElement | null>>({});
    const [activeTop, setActiveTop] = useState(0);
    const [activeHeight, setActiveHeight] = useState(48);

    const activeIndex = useMemo(() => FEATURES.findIndex(f => f.id === active), [active]);
    const isValidActive = activeIndex !== -1;

    useLayoutEffect(() => {
        const listEl = listRef.current;
        const btn = itemRefs.current[active] ?? null;
        if (!listEl || !btn) return;

        // Ensure we calculate offset relative to the scrolling container
        const top = btn.offsetTop;
        const height = btn.offsetHeight;
        
        setActiveTop(top);
        setActiveHeight(height);
    }, [active, collapsed, activeIndex]);

    return (
        <div 
            className={`${collapsed ? 'w-20' : 'w-64'} h-full bg-white/80 dark:bg-[#1C1C1E]/80 glass border-r border-gray-200 dark:border-white/5 flex flex-col transition-[width] duration-500 ease-[cubic-bezier(0.2,0.8,0.2,1)] relative z-20 shrink-0`}
        >
            {/* Navigation Items */}
            <div ref={listRef} className="flex-1 overflow-y-auto px-3 space-y-1 no-scrollbar py-4 relative">
                <div className={`h-6 text-xs font-medium text-gray-400 px-3 uppercase tracking-wider transition-all duration-300 flex items-center whitespace-nowrap overflow-hidden ${collapsed ? 'opacity-0' : 'opacity-100'}`}>
                    工具箱
                </div>

                <div
                    className={`absolute top-0 left-3 right-3 bg-[#007AFF] rounded-xl shadow-md shadow-blue-500/20 transition-all duration-300 ease-[cubic-bezier(0.2,0.8,0.2,1)] pointer-events-none ${
                        isValidActive ? 'opacity-100 scale-100' : 'opacity-0 scale-95'
                    }`}
                    style={{
                        height: activeHeight,
                        transform: `translateY(${activeTop}px)`,
                    }}
                />
                
                {FEATURES.map((item) => (
                    <button
                        key={item.id}
                        ref={el => { itemRefs.current[item.id] = el; }}
                        onClick={() => setActive(item.id)}
                        title={collapsed ? item.title : ''}
                        className={`w-full flex items-center h-12 rounded-xl text-sm font-medium transition-all duration-300 group relative px-3 z-10 ${
                            active === item.id 
                                ? 'bg-transparent'
                                : 'text-gray-600 dark:text-gray-300 hover:bg-black/5 dark:hover:bg-white/10'
                        }`}
                    >
                        {/* Icon Wrapper: Fixed width ensures icon position never jumps */}
                        <div className="w-6 h-6 flex items-center justify-center shrink-0">
                            <Icon name={item.iconName} size={20} className={`${active === item.id ? 'text-white' : item.color} transition-colors duration-200`} />
                        </div>
                        
                        {/* Text Wrapper */}
                        <div className={`whitespace-nowrap overflow-hidden transition-all duration-500 ease-[cubic-bezier(0.2,0.8,0.2,1)] ${collapsed ? 'w-0 opacity-0' : 'w-40 opacity-100'}`}>
                            <span className={`ml-3 block transition-colors duration-200 ${active === item.id ? 'text-white font-semibold' : ''}`}>
                                {item.title}
                            </span>
                        </div>

                        {/* Hover Tooltip for Collapsed State */}
                        {collapsed && (
                            <div className="absolute left-14 top-1/2 -translate-y-1/2 px-3 py-1.5 bg-gray-900 dark:bg-white text-white dark:text-gray-900 text-xs rounded-lg shadow-xl opacity-0 group-hover:opacity-100 transition-all duration-200 translate-x-2 group-hover:translate-x-0 pointer-events-none z-50 whitespace-nowrap">
                                {item.title}
                            </div>
                        )}
                    </button>
                ))}
            </div>

            {/* Footer / Toggle & Settings */}
            <div className="p-3 border-t border-gray-200 dark:border-white/5 space-y-1">
                <button 
                    onClick={() => setCollapsed(!collapsed)}
                    className="w-full flex items-center h-10 rounded-xl text-sm font-medium text-gray-500 dark:text-gray-400 hover:bg-black/5 dark:hover:bg-white/10 transition-colors group px-3"
                >
                    <div className="w-6 h-6 flex items-center justify-center shrink-0">
                        <Icon name={collapsed ? "PanelLeft" : "PanelLeftOpen"} size={20} className="transition-transform duration-300 group-hover:scale-110" />
                    </div>
                    <div className={`whitespace-nowrap overflow-hidden transition-all duration-500 ease-[cubic-bezier(0.2,0.8,0.2,1)] ${collapsed ? 'w-0 opacity-0' : 'w-40 opacity-100'}`}>
                        <span className="ml-3">收起侧边栏</span>
                    </div>
                </button>

                <button
                    onClick={() => setActive('settings')}
                    className="w-full flex items-center h-10 rounded-xl text-sm font-medium text-gray-600 dark:text-gray-300 hover:bg-black/5 dark:hover:bg-white/10 transition-colors px-3"
                >
                    <div className="w-6 h-6 flex items-center justify-center shrink-0">
                        <Icon name="Settings" size={20} />
                    </div>
                    <div className={`whitespace-nowrap overflow-hidden transition-all duration-500 ease-[cubic-bezier(0.2,0.8,0.2,1)] ${collapsed ? 'w-0 opacity-0' : 'w-40 opacity-100'}`}>
                        <span className="ml-3">全局设置</span>
                    </div>
                </button>
            </div>
        </div>
    );
};

export default Sidebar;
