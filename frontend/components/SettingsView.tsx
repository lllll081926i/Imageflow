import React, { useEffect, useMemo, useState } from 'react';
import Icon from './Icon';
import { Switch } from './Controls';
import {
    DEFAULT_APP_SETTINGS,
    getAppBindings,
    loadAppSettings,
    saveAppSettings,
    type AppSettingsSnapshot,
} from '../types/wails-api';

const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, value));

const SettingsView: React.FC = () => {
    const [settings, setSettings] = useState<AppSettingsSnapshot>({ ...DEFAULT_APP_SETTINGS });
    const [saving, setSaving] = useState(false);
    const [loading, setLoading] = useState(true);
    const [message, setMessage] = useState('');

    const maxConcurrency = useMemo(
        () => clamp(settings.max_concurrency || DEFAULT_APP_SETTINGS.max_concurrency, 1, 32),
        [settings.max_concurrency],
    );

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
                const loaded = await loadAppSettings({ throwOnError: true });
                setSettings(loaded);
            } catch (error: any) {
                console.error(error);
                setSettings({ ...DEFAULT_APP_SETTINGS });
                setMessage(error?.message ? `读取设置失败：${error.message}` : '读取设置失败，已使用默认值');
            } finally {
                setLoading(false);
            }
        };
        void run();
    }, []);

    const handleSave = async (next?: AppSettingsSnapshot) => {
        setSaving(true);
        setMessage('');
        try {
            const saved = await saveAppSettings(next || settings);
            setSettings(saved);
            setMessage('设置已保存');
            window.setTimeout(() => setMessage(''), 1800);
        } catch (error: any) {
            console.error(error);
            setMessage(error?.message ? `保存失败：${error.message}` : '保存失败');
        } finally {
            setSaving(false);
        }
    };

    const handleSelectDefaultOutputDir = async () => {
        const app = getAppBindings();
        if (!app?.SelectOutputDirectory) {
            setMessage('当前环境不支持目录选择');
            return;
        }
        try {
            const selected = await app.SelectOutputDirectory();
            if (typeof selected === 'string' && selected.trim()) {
                setSettings((previous) => ({ ...previous, default_output_dir: selected.trim() }));
            }
        } catch (error: any) {
            console.error(error);
            setMessage(error?.message ? `选择目录失败：${error.message}` : '选择目录失败');
        }
    };

    const recentInputDirs = settings.recent_input_dirs || [];
    const recentOutputDirs = settings.recent_output_dirs || [];

    return (
        <div className="h-full flex flex-col min-h-0">
            <div className="flex items-start justify-between gap-4 mb-6">
                <div className="flex items-center gap-3">
                    <div className="w-11 h-11 rounded-2xl bg-white dark:bg-[#2C2C2E] border border-gray-200 dark:border-white/10 shadow-sm flex items-center justify-center">
                        <Icon name="Settings" size={20} className="text-[#007AFF]" />
                    </div>
                    <div>
                        <div className="text-xl font-semibold text-gray-900 dark:text-white">全局设置</div>
                        <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">统一管理默认输出、路径记录和处理规则</div>
                    </div>
                </div>
                <div className="flex items-center gap-3">
                    <div className="text-xs text-gray-500 dark:text-gray-400 min-w-[90px] text-right">
                        {loading
                            ? '读取中...'
                            : (message ? <span className={message.includes('失败') ? 'text-red-500' : 'text-[#007AFF]'}>{message}</span> : '已就绪')}
                    </div>
                    <button
                        type="button"
                        disabled={saving || loading}
                        onClick={() => void handleSave({ ...settings, max_concurrency: maxConcurrency })}
                        className={`px-4 py-2.5 rounded-xl text-sm font-semibold transition-colors ${(saving || loading) ? 'bg-gray-200 dark:bg-white/10 text-gray-500 dark:text-gray-400 cursor-not-allowed' : 'bg-[#007AFF] text-white hover:bg-[#005ED0]'}`}
                    >
                        {saving ? '保存中...' : '保存全部'}
                    </button>
                </div>
            </div>

            <div className="flex-1 overflow-y-auto pr-1">
                <div className="grid grid-cols-1 xl:grid-cols-[1.15fr_0.85fr] gap-6">
                    <div className="space-y-6">
                        <section className="bg-white dark:bg-[#2C2C2E] rounded-3xl border border-gray-200 dark:border-white/10 shadow-sm p-6">
                            <div className="flex items-start justify-between gap-4 mb-5">
                                <div>
                                    <div className="text-base font-semibold text-gray-900 dark:text-white">常用路径</div>
                                    <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">默认输出目录会自动带入处理页面，最近路径只保留最新 4 条</div>
                                </div>
                                <button
                                    type="button"
                                    disabled={saving || loading || (recentInputDirs.length === 0 && recentOutputDirs.length === 0)}
                                    onClick={() => void handleSave({ ...settings, recent_input_dirs: [], recent_output_dirs: [] })}
                                    className="px-3 py-2 rounded-xl text-sm border border-gray-200 dark:border-white/10 text-gray-600 dark:text-gray-300 hover:text-red-500 hover:border-red-200 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    清空最近路径
                                </button>
                            </div>

                            <div className="space-y-5">
                                <div className="space-y-2">
                                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">默认输出目录</label>
                                    <div className="flex items-center gap-2">
                                        <input
                                            type="text"
                                            value={settings.default_output_dir}
                                            onChange={(event) => setSettings((previous) => ({ ...previous, default_output_dir: event.target.value }))}
                                            placeholder="未设置时，处理页将沿用当前任务目录"
                                            className="flex-1 px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white"
                                        />
                                        <button
                                            type="button"
                                            onClick={() => void handleSelectDefaultOutputDir()}
                                            className="px-3 py-2.5 rounded-xl border border-gray-200 dark:border-white/10 text-sm text-gray-600 dark:text-gray-300 hover:border-[#007AFF] hover:text-[#007AFF] transition-colors"
                                        >
                                            浏览
                                        </button>
                                    </div>
                                </div>

                                <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                                    <div className="rounded-2xl bg-gray-50 dark:bg-white/5 border border-gray-200/80 dark:border-white/10 p-4">
                                        <div className="text-sm font-medium text-gray-800 dark:text-gray-100 mb-3">最近输入路径</div>
                                        <div className="space-y-2">
                                            {recentInputDirs.length === 0 ? (
                                                <div className="text-sm text-gray-400">暂无记录</div>
                                            ) : recentInputDirs.map((path, index) => (
                                                <div key={`input-${path}-${index}`} className="rounded-xl bg-white dark:bg-white/5 border border-gray-200/70 dark:border-white/10 px-3 py-2 text-sm text-gray-600 dark:text-gray-300 break-all">
                                                    {path}
                                                </div>
                                            ))}
                                        </div>
                                    </div>

                                    <div className="rounded-2xl bg-gray-50 dark:bg-white/5 border border-gray-200/80 dark:border-white/10 p-4">
                                        <div className="text-sm font-medium text-gray-800 dark:text-gray-100 mb-3">最近输出路径</div>
                                        <div className="space-y-2">
                                            {recentOutputDirs.length === 0 ? (
                                                <div className="text-sm text-gray-400">暂无记录</div>
                                            ) : recentOutputDirs.map((path, index) => (
                                                <button
                                                    key={`output-${path}-${index}`}
                                                    type="button"
                                                    onClick={() => {
                                                        setSettings((previous) => ({ ...previous, default_output_dir: path }));
                                                        setMessage('已填入默认输出目录，记得保存全部');
                                                    }}
                                                    className="w-full text-left rounded-xl bg-white dark:bg-white/5 border border-gray-200/70 dark:border-white/10 px-3 py-2 text-sm text-gray-600 dark:text-gray-300 break-all hover:border-[#007AFF]/40 hover:text-[#007AFF] transition-colors"
                                                    title="点击设为默认输出目录"
                                                >
                                                    {path}
                                                </button>
                                            ))}
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </section>

                        <section className="bg-white dark:bg-[#2C2C2E] rounded-3xl border border-gray-200 dark:border-white/10 shadow-sm p-6">
                            <div className="mb-5">
                                <div className="text-base font-semibold text-gray-900 dark:text-white">输出规则</div>
                                <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">这里定义批量处理时的默认命名方式</div>
                            </div>

                            <div className="space-y-4">
                                <div className="space-y-2">
                                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">命名模板</label>
                                    <input
                                        type="text"
                                        value={settings.output_template}
                                        onChange={(event) => setSettings((previous) => ({ ...previous, output_template: event.target.value }))}
                                        placeholder="{prefix}{basename}_{date:YYYYMMDD}_{seq:3}"
                                        className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white"
                                    />
                                </div>

                                <div className="space-y-2">
                                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">默认前缀</label>
                                    <input
                                        type="text"
                                        value={settings.output_prefix}
                                        onChange={(event) => setSettings((previous) => ({ ...previous, output_prefix: event.target.value }))}
                                        placeholder="例如：IF"
                                        className="w-full px-3 py-2.5 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-sm focus:border-[#007AFF] focus:ring-1 focus:ring-[#007AFF] outline-none dark:text-white"
                                    />
                                </div>

                                <div className="rounded-2xl bg-gray-50 dark:bg-white/5 border border-gray-200/80 dark:border-white/10 p-4 space-y-3">
                                    <Switch
                                        checked={settings.preserve_folder_structure}
                                        onChange={(checked) => setSettings((previous) => ({ ...previous, preserve_folder_structure: checked }))}
                                        label="保持原文件夹结构"
                                    />
                                    <div className="flex items-center justify-between gap-4 text-sm">
                                        <span className="text-gray-500 dark:text-gray-400">重名文件处理</span>
                                        <span className="px-2.5 py-1 rounded-full bg-[#007AFF]/10 text-[#007AFF]">自动重命名</span>
                                    </div>
                                </div>
                            </div>
                        </section>
                    </div>

                    <div className="space-y-6">
                        <section className="bg-white dark:bg-[#2C2C2E] rounded-3xl border border-gray-200 dark:border-white/10 shadow-sm p-6">
                            <div className="mb-5">
                                <div className="text-base font-semibold text-gray-900 dark:text-white">处理性能</div>
                                <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">控制批量任务的并发上限，影响速度与资源占用</div>
                            </div>

                            <div className="flex items-center justify-between gap-3 mb-4">
                                <div className="text-sm font-medium text-gray-700 dark:text-gray-300">全局并发数</div>
                                <input
                                    type="number"
                                    min={1}
                                    max={32}
                                    value={maxConcurrency}
                                    onChange={(event) => setSettings((previous) => ({
                                        ...previous,
                                        max_concurrency: clamp(Number(event.target.value || 1), 1, 32),
                                    }))}
                                    className="w-20 text-center text-gray-700 dark:text-gray-200 font-mono text-sm bg-gray-100 dark:bg-white/10 px-3 py-2 rounded-xl outline-none focus:ring-2 focus:ring-[#007AFF]/30 border border-transparent focus:border-[#007AFF]"
                                />
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
                                    onChange={(event) => setSettings((previous) => ({
                                        ...previous,
                                        max_concurrency: clamp(Number(event.target.value), 1, 32),
                                    }))}
                                    className="absolute inset-0 w-full h-full opacity-0 cursor-pointer z-30"
                                />
                            </div>
                        </section>

                        <section className="bg-white dark:bg-[#2C2C2E] rounded-3xl border border-gray-200 dark:border-white/10 shadow-sm p-6">
                            <div className="mb-5">
                                <div className="text-base font-semibold text-gray-900 dark:text-white">当前规则摘要</div>
                                <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">方便快速确认全局设置是否符合当前工作流</div>
                            </div>

                            <div className="space-y-3 text-sm">
                                <div className="flex items-start justify-between gap-4">
                                    <span className="text-gray-500 dark:text-gray-400">默认输出目录</span>
                                    <span className="text-right text-gray-800 dark:text-gray-100 break-all">{settings.default_output_dir || '未设置'}</span>
                                </div>
                                <div className="flex items-start justify-between gap-4">
                                    <span className="text-gray-500 dark:text-gray-400">命名模板</span>
                                    <span className="text-right text-gray-800 dark:text-gray-100 break-all">{settings.output_template}</span>
                                </div>
                                <div className="flex items-start justify-between gap-4">
                                    <span className="text-gray-500 dark:text-gray-400">默认前缀</span>
                                    <span className="text-right text-gray-800 dark:text-gray-100">{settings.output_prefix}</span>
                                </div>
                                <div className="flex items-start justify-between gap-4">
                                    <span className="text-gray-500 dark:text-gray-400">目录结构</span>
                                    <span className="text-right text-gray-800 dark:text-gray-100">{settings.preserve_folder_structure ? '保持原结构' : '平铺输出'}</span>
                                </div>
                                <div className="flex items-start justify-between gap-4">
                                    <span className="text-gray-500 dark:text-gray-400">最近路径数量</span>
                                    <span className="text-right text-gray-800 dark:text-gray-100">输入 {recentInputDirs.length} 条 / 输出 {recentOutputDirs.length} 条</span>
                                </div>
                                <div className="flex items-start justify-between gap-4">
                                    <span className="text-gray-500 dark:text-gray-400">并发上限</span>
                                    <span className="text-right text-gray-800 dark:text-gray-100">{maxConcurrency}</span>
                                </div>
                            </div>
                        </section>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default SettingsView;
