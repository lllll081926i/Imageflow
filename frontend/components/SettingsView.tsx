import React, { useEffect, useMemo, useState } from 'react';
import Icon from './Icon';
import { Switch } from './Controls';
import { getAppBindings } from '../types/wails-api';

type AppSettings = {
    max_concurrency: number;
    output_prefix: string;
    output_template: string;
    preserve_folder_structure: boolean;
    conflict_strategy: string;
};

const clamp = (v: number, min: number, max: number) => Math.max(min, Math.min(max, v));
const defaultSettings: AppSettings = {
    max_concurrency: 8,
    output_prefix: 'IF',
    output_template: '{prefix}{basename}',
    preserve_folder_structure: true,
    conflict_strategy: 'rename',
};

const SettingsView: React.FC = () => {
    const [settings, setSettings] = useState<AppSettings>(defaultSettings);
    const [saving, setSaving] = useState(false);
    const [loading, setLoading] = useState(true);
    const [message, setMessage] = useState<string>('');

    const maxConcurrency = useMemo(() => clamp(settings.max_concurrency || 8, 1, 32), [settings.max_concurrency]);

    useEffect(() => {
        const run = async () => {
            setLoading(true);
            const app = getAppBindings();
            if (!app?.GetSettings) {
                setMessage('未检测到 Wails 运行环境');
                setLoading(false);
                return;
            }
            try {
                const res = await app.GetSettings();
                if (res) {
                    setSettings({
                        max_concurrency: clamp(Number(res.max_concurrency || defaultSettings.max_concurrency), 1, 32),
                        output_prefix: (typeof res.output_prefix === 'string' ? res.output_prefix : defaultSettings.output_prefix),
                        output_template: (typeof res.output_template === 'string' ? res.output_template : defaultSettings.output_template),
                        preserve_folder_structure: typeof res.preserve_folder_structure === 'boolean'
                            ? res.preserve_folder_structure
                            : defaultSettings.preserve_folder_structure,
                        conflict_strategy: (typeof res.conflict_strategy === 'string' && res.conflict_strategy === 'rename')
                            ? 'rename'
                            : defaultSettings.conflict_strategy,
                    });
                }
            } catch (e: any) {
                console.error(e);
                setMessage(e?.message ? `读取设置失败：${e.message}` : '读取设置失败，已使用默认值');
            } finally {
                setLoading(false);
            }
        };
        run();
    }, []);

    const save = async (next: AppSettings) => {
        const app = getAppBindings();
        if (!app?.SaveSettings) {
            setMessage('未检测到 Wails 运行环境');
            return;
        }
        setSaving(true);
        setMessage('');
        try {
            const saved = await app.SaveSettings(next);
            if (saved && typeof saved.max_concurrency === 'number') {
                setSettings({
                    max_concurrency: clamp(Number(saved.max_concurrency || defaultSettings.max_concurrency), 1, 32),
                    output_prefix: (typeof saved.output_prefix === 'string' ? saved.output_prefix : next.output_prefix),
                    output_template: (typeof saved.output_template === 'string' ? saved.output_template : next.output_template),
                    preserve_folder_structure: typeof saved.preserve_folder_structure === 'boolean'
                        ? saved.preserve_folder_structure
                        : next.preserve_folder_structure,
                    conflict_strategy: (typeof saved.conflict_strategy === 'string' && saved.conflict_strategy === 'rename')
                        ? 'rename'
                        : next.conflict_strategy,
                });
            } else {
                setSettings({
                    max_concurrency: clamp(next.max_concurrency, 1, 32),
                    output_prefix: next.output_prefix,
                    output_template: next.output_template,
                    preserve_folder_structure: next.preserve_folder_structure,
                    conflict_strategy: next.conflict_strategy,
                });
            }
            setMessage('已保存');
            setTimeout(() => setMessage(''), 1500);
        } catch (e: any) {
            console.error(e);
            setMessage(e?.message ? `保存失败：${e.message}` : '保存失败');
        } finally {
            setSaving(false);
        }
    };

    return (
        <div className="h-full flex flex-col">
            <div className="flex items-center justify-between mb-6">
                <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-2xl bg-white dark:bg-[#2C2C2E] border border-gray-200 dark:border-white/10 shadow-sm flex items-center justify-center">
                        <Icon name="Settings" size={20} className="text-[#007AFF]" />
                    </div>
                    <div>
                        <div className="text-xl font-semibold text-gray-900 dark:text-white">全局设置</div>
                    </div>
                </div>
                <div className="text-xs text-gray-500 dark:text-gray-400">
                    {loading
                        ? <span>读取中...</span>
                        : (message && <span className={message.includes('失败') ? 'text-red-500' : 'text-[#007AFF]'}>{message}</span>)}
                </div>
            </div>

            <div className="flex-1 grid grid-cols-1 lg:grid-cols-2 gap-6 min-h-0">
                <div className="bg-white dark:bg-[#2C2C2E] rounded-3xl border border-gray-200 dark:border-white/10 shadow-sm p-6 flex flex-col min-h-0">
                    <div className="flex items-center justify-between mb-5">
                        <div>
                            <div className="text-sm font-semibold text-gray-900 dark:text-white">全局并发数</div>
                        </div>
                        <div className="flex items-center gap-2">
                            <input
                                type="number"
                                min={1}
                                max={32}
                                value={maxConcurrency}
                                onChange={(e) => setSettings(s => ({ ...s, max_concurrency: clamp(Number(e.target.value || 1), 1, 32) }))}
                                className="w-20 text-center text-gray-700 dark:text-gray-200 font-mono text-sm bg-gray-100 dark:bg-white/10 px-3 py-2 rounded-xl outline-none focus:ring-2 focus:ring-[#007AFF]/30 border border-transparent focus:border-[#007AFF]"
                            />
                            <button
                                disabled={saving || loading}
                                onClick={() => save({
                                    ...settings,
                                    max_concurrency: maxConcurrency,
                                    conflict_strategy: 'rename',
                                })}
                                className={`px-4 py-2 rounded-xl text-sm font-semibold transition-colors ${(saving || loading) ? 'bg-gray-200 dark:bg-white/10 text-gray-500 dark:text-gray-400 cursor-not-allowed' : 'bg-[#007AFF] text-white hover:bg-[#005ED0]'}`}
                            >
                                {saving ? '保存中...' : '保存'}
                            </button>
                        </div>
                    </div>

                    <div className="relative h-10 w-full flex items-center">
                        <div className="absolute w-full h-2 bg-gray-200 dark:bg-white/10 rounded-full overflow-hidden">
                            <div className="h-full bg-[#007AFF]" style={{ width: `${((maxConcurrency - 1) / 31) * 100}%` }} />
                        </div>
                        <div
                            className="absolute h-6 w-6 bg-white dark:bg-[#1C1C1E] shadow-[0_2px_4px_rgba(0,0,0,0.18)] border border-gray-200 dark:border-white/10 rounded-full flex items-center justify-center pointer-events-none z-20"
                            style={{ left: `${((maxConcurrency - 1) / 31) * 100}%`, transform: 'translateX(-50%)' }}
                        >
                            <div className="w-2 h-2 bg-[#007AFF] rounded-full" />
                        </div>
                        <input
                            type="range"
                            min={1}
                            max={32}
                            step={1}
                            value={maxConcurrency}
                            onChange={(e) => setSettings(s => ({ ...s, max_concurrency: clamp(Number(e.target.value), 1, 32) }))}
                            className="absolute inset-0 w-full h-full opacity-0 cursor-pointer z-30"
                        />
                    </div>
                </div>

                <div className="bg-white dark:bg-[#2C2C2E] rounded-3xl border border-gray-200 dark:border-white/10 shadow-sm p-6 flex flex-col min-h-0">
                    <div className="text-sm font-semibold text-gray-900 dark:text-white mb-4">输出规则（全局）</div>

                    <div className="space-y-2">
                        <label className="text-sm font-medium text-gray-700 dark:text-gray-300">命名模板</label>
                        <input
                            type="text"
                            value={settings.output_template}
                            onChange={(e) => setSettings(s => ({ ...s, output_template: e.target.value }))}
                            placeholder="{prefix}{basename}_{date:YYYYMMDD}_{seq:3}"
                            className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white"
                        />
                    </div>

                    <div className="mt-4 space-y-2">
                        <label className="text-sm font-medium text-gray-700 dark:text-gray-300">默认前缀</label>
                        <input
                            type="text"
                            value={settings.output_prefix}
                            onChange={(e) => setSettings(s => ({ ...s, output_prefix: e.target.value }))}
                            placeholder="例如: IF"
                            className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white"
                        />
                    </div>

                    <div className="mt-4 space-y-3">
                        <Switch
                            checked={settings.preserve_folder_structure}
                            onChange={(checked) => setSettings(s => ({ ...s, preserve_folder_structure: checked }))}
                            label="保持原文件夹结构"
                        />
                    </div>

                    <div className="mt-4">
                        <button
                            disabled={saving || loading}
                            onClick={() => save({
                                ...settings,
                                max_concurrency: maxConcurrency,
                                conflict_strategy: 'rename',
                            })}
                            className={`w-full px-4 py-2.5 rounded-xl text-sm font-semibold transition-colors ${(saving || loading) ? 'bg-gray-200 dark:bg-white/10 text-gray-500 dark:text-gray-400 cursor-not-allowed' : 'bg-[#007AFF] text-white hover:bg-[#005ED0]'}`}
                        >
                            {saving ? '保存中...' : '保存输出规则'}
                        </button>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default SettingsView;
