import React from 'react';
import Icon from './Icon';
import { FEATURES } from '../constants';
import { ViewState } from '../types';

interface DashboardProps {
    onSelect: (view: ViewState) => void;
    onPreload?: (view: ViewState) => void;
}

const Dashboard: React.FC<DashboardProps> = ({ onSelect, onPreload }) => {
    return (
        <div>
            <div className="mb-8 mt-4">
                <h1 className="text-3xl font-bold text-gray-900 dark:text-white mb-2 tracking-tight">欢迎回来</h1>
                <p className="text-gray-500 dark:text-gray-400">选择一个工具开始高效处理图片</p>
            </div>
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 gap-6 pb-8">
                {FEATURES.map((item) => (
                    <button
                        type="button"
                        key={item.id}
                        onClick={() => onSelect(item.id)} 
                        onPointerEnter={() => onPreload?.(item.id)}
                        onFocus={() => onPreload?.(item.id)}
                        className="group relative bg-white dark:bg-[#2C2C2E] p-6 rounded-2xl shadow-sm hover:shadow-xl hover:-translate-y-1 transition-all duration-300 cursor-pointer border border-transparent hover:border-gray-100 dark:hover:border-white/5 text-left"
                    >
                        <div className={`w-14 h-14 rounded-2xl ${item.bg} ${item.darkBg} flex items-center justify-center mb-4 transition-transform group-hover:scale-110 duration-300`}>
                            <Icon name={item.iconName} className={`w-7 h-7 ${item.color} dark:text-white`} />
                        </div>
                        <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-1">{item.title}</h3>
                        <p className="text-sm text-gray-500 dark:text-gray-400">{item.desc}</p>
                        <div className="absolute top-6 right-6 opacity-0 group-hover:opacity-100 transition-opacity duration-300 transform translate-x-2 group-hover:translate-x-0">
                            <Icon name="ArrowRight" className="w-5 h-5 text-gray-300 dark:text-gray-500" />
                        </div>
                    </button>
                ))}
            </div>
        </div>
    );
};

export default Dashboard;
