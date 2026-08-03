import React, { useCallback, useEffect, useMemo, useState } from 'react';
import Icon from './Icon';
import { Switch } from './Controls';
import {
    DEFAULT_APP_SETTINGS,
    getAppBindings,
    loadAppSettings,
    saveAppSettings,
    type AppSettingsSnapshot,
} from '../types/app-api';

const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, value));

type UpdateCheckResult = {
    success?: boolean;
    current_version?: string;
    latest_version?: string;
    update_available?: boolean;
    release_url?: string;
    release_notes?: string;
    checked?: boolean;
    error?: string;
};

const Section: React.FC<{
    title: string;
    desc: string;
    icon: string;
    children: React.ReactNode;
    action?: React.ReactNode;
}> = ({ title, desc, icon, children, action }) => (
    <section className="bg-white dark:bg-[#2C2C2E] rounded-3xl border border-gray-200 dark:border-white/10 shadow-sm p-6">
        <div className="flex items-start justify-between gap-4 mb-5">
            <div className="flex items-start gap-3 min-w-0">
                <div className="w-10 h-10 rounded-2xl bg-[#007AFF]/10 text-[#007AFF] flex items-center justify-center shrink-0">
                    <Icon name={icon as any} size={18} />
                </div>
                <div className="min-w-0">
                    <div className="text-base font-semibold text-gray-900 dark:text-white">{title}</div>
                    <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">{desc}</div>
                </div>
            </div>
            {action}
        </div>
        {children}
    </section>
);

const SettingsView: React.FC = () => {
    const [settings, setSettings] = useState<AppSettingsSnapshot>({ ...DEFAULT_APP_SETTINGS });
    const [saving, setSaving] = useState(false);
    const [loading, setLoading] = useState(true);
    const [message, setMessage] = useState('');
    const [appVersion, setAppVersion] = useState('');
    const [updateBusy, setUpdateBusy] = useState(false);
    const [updateInfo, setUpdateInfo] = useState<UpdateCheckResult | null>(null);

    const maxConcurrency = useMemo(
        () => clamp(settings.max_concurrency || DEFAULT_APP_SETTINGS.max_concurrency, 1, 32),
        [settings.max_concurrency],
    );

    const loadVersion = useCallback(async () => {
        const app = getAppBindings() as any;
        try {
            if (app?.GetAppVersion) {
                const res = await app.GetAppVersion();
                if (res?.version) setAppVersion(String(res.version));
            }
        } catch {
            // ignore
        }
    }, []);

    const runUpdateCheck = useCallback(async (silent = false) => {
        const app = getAppBindings() as any;
        if (!app?.CheckForUpdates) {
            if (!silent) setMessage('当前环境不支持检查更新');
            return;
        }
        setUpdateBusy(true);
        if (!silent) setMessage('');
        try {
            const res = (await app.CheckForUpdates({ timeout_seconds: 8 })) as UpdateCheckResult;
            setUpdateInfo(res || null);
            if (res?.current_version) setAppVersion(String(res.current_version));
            if (!silent) {
                if (res?.update_available) {
                    setMessage(`发现新版本 ${res.latest_version}`);
                } else if (res?.checked) {
                    setMessage('已是最新版本');
                } else if (res?.error) {
                    setMessage(`检查更新失败：${res.error}`);
                } else {
                    setMessage('检查更新完成');
                }
                window.setTimeout(() => setMessage(''), 2200);
            }
        } catch (error: any) {
            if (!silent) setMessage(error?.message ? `检查更新失败：${error.message}` : '检查更新失败');
        } finally {
            setUpdateBusy(false);
        }
    }, []);

    useEffect(() => {
        const run = async () => {
            setLoading(true);
            const app = getAppBindings();
            if (!app?.GetSettings) {
                setMessage('未检测到桌面宿主运行环境');
                setLoading(false);
                return;
            }
            try {
                const loaded = await loadAppSettings({ throwOnError: true });
                setSettings(loaded);
                await loadVersion();
                if (loaded.auto_check_updates) {
                    void runUpdateCheck(true);
                }
            } catch (error: any) {
                console.error(error);
                setSettings({ ...DEFAULT_APP_SETTINGS });
                setMessage(error?.message ? `读取设置失败：${error.message}` : '读取设置失败，已使用默认值');
            } finally {
                setLoading(false);
            }
        };
        void run();
    }, [loadVersion, runUpdateCheck]);

    const handleSave = async (next?: AppSettingsSnapshot) => {
        setSaving(true);
        setMessage('');
        try {
            const payload = next || { ...settings, max_concurrency: maxConcurrency };
            const saved = await saveAppSettings(payload);
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

    const openReleasePage = () => {
        const url = updateInfo?.release_url || 'https://github.com/lllll081926i/Imageflow/releases';
        try {
            window.open(url, '_blank', 'noopener,noreferrer');
        } catch {
            setMessage('无法打开发布页');
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
                        <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">
                            路径、输出规则、性能与更新
                            {appVersion ? ` · v${appVersion}` : ''}
                        </div>
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
                <div className="grid grid-cols-1 xl:grid-cols-2 gap-6">
                    <div className="space-y-6">
                        <Section
                            title="常用路径"
                            desc="默认输出目录会自动带入处理页，最近路径最多保留 4 条"
                            icon="Upload"
                            action={(
                                <button
                                    type="button"
                                    disabled={saving || loading || (recentInputDirs.length === 0 && recentOutputDirs.length === 0)}
                                    onClick={() => void handleSave({ ...settings, recent_input_dirs: [], recent_output_dirs: [] })}
                                    className="px-3 py-2 rounded-xl text-sm border border-gray-200 dark:border-white/10 text-gray-600 dark:text-gray-300 hover:text-red-500 hover:border-red-200 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
                                >
                                    清空最近
                                </button>
                            )}
                        >
                            <div className="space-y-5">
                                <div className="space-y-2">
                                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">默认输出目录</label>
                                    <div className="flex items-center gap-2">
                                        <input
                                            type="text"
                                            value={settings.default_output_dir}
                                            onChange={(event) => setSettings((previous) => ({ ...previous, default_output_dir: event.target.value }))}
                                            placeholder="未设置时沿用当前任务目录"
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
                                        <div className="text-sm font-medium text-gray-800 dark:text-gray-100 mb-3">最近输入</div>
                                        <div className="space-y-2 max-h-40 overflow-y-auto">
                                            {recentInputDirs.length === 0 ? (
                                                <div className="text-sm text-gray-400">暂无记录</div>
                                            ) : recentInputDirs.map((path, index) => (
                                                <div key={`input-${path}-${index}`} className="rounded-xl bg-white dark:bg-white/5 border border-gray-200/70 dark:border-white/10 px-3 py-2 text-xs text-gray-600 dark:text-gray-300 break-all">
                                                    {path}
                                                </div>
                                            ))}
                                        </div>
                                    </div>
                                    <div className="rounded-2xl bg-gray-50 dark:bg-white/5 border border-gray-200/80 dark:border-white/10 p-4">
                                        <div className="text-sm font-medium text-gray-800 dark:text-gray-100 mb-3">最近输出</div>
                                        <div className="space-y-2 max-h-40 overflow-y-auto">
                                            {recentOutputDirs.length === 0 ? (
                                                <div className="text-sm text-gray-400">暂无记录</div>
                                            ) : recentOutputDirs.map((path, index) => (
                                                <button
                                                    key={`output-${path}-${index}`}
                                                    type="button"
                                                    onClick={() => {
                                                        setSettings((previous) => ({ ...previous, default_output_dir: path }));
                                                        setMessage('已填入默认输出目录，记得保存');
                                                    }}
                                                    className="w-full text-left rounded-xl bg-white dark:bg-white/5 border border-gray-200/70 dark:border-white/10 px-3 py-2 text-xs text-gray-600 dark:text-gray-300 break-all hover:border-[#007AFF]/40 hover:text-[#007AFF] transition-colors"
                                                >
                                                    {path}
                                                </button>
                                            ))}
                                        </div>
                                    </div>
                                </div>
                            </div>
                        </Section>

                        <Section title="输出规则" desc="批量命名、冲突策略与目录结构" icon="FileType">
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
                                    <div className="text-[11px] text-gray-400">可用变量：{'{prefix} {basename} {date:YYYYMMDD} {seq:3} {op}'}</div>
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
                                        <span className="text-gray-500 dark:text-gray-400">重名文件</span>
                                        <span className="px-2.5 py-1 rounded-full bg-[#007AFF]/10 text-[#007AFF] text-xs font-medium">自动重命名</span>
                                    </div>
                                    <Switch
                                        checked={Boolean(settings.open_output_folder_on_complete)}
                                        onChange={(checked) => setSettings((previous) => ({ ...previous, open_output_folder_on_complete: checked }))}
                                        label="处理完成后打开输出目录"
                                    />
                                </div>
                            </div>
                        </Section>
                    </div>

                    <div className="space-y-6">
                        <Section title="处理性能" desc="控制批量任务并发，数值越大越吃 CPU / 内存" icon="Layers">
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
                            <div className="mt-3 text-xs text-gray-400">Windows 建议 2–4；机器性能好可适当提高。</div>
                        </Section>

                        <Section
                            title="软件更新"
                            desc="从 GitHub Releases 检查新版本"
                            icon="ArrowRight"
                            action={(
                                <button
                                    type="button"
                                    disabled={updateBusy || loading}
                                    onClick={() => void runUpdateCheck(false)}
                                    className="px-3 py-2 rounded-xl text-sm border border-gray-200 dark:border-white/10 text-gray-700 dark:text-gray-200 hover:border-[#007AFF] hover:text-[#007AFF] transition-colors disabled:opacity-50"
                                >
                                    {updateBusy ? '检查中…' : '立即检查'}
                                </button>
                            )}
                        >
                            <div className="space-y-4">
                                <Switch
                                    checked={Boolean(settings.auto_check_updates)}
                                    onChange={(checked) => setSettings((previous) => ({ ...previous, auto_check_updates: checked }))}
                                    label="打开设置时自动检查更新"
                                />
                                <div className="rounded-2xl bg-gray-50 dark:bg-white/5 border border-gray-200/80 dark:border-white/10 p-4 space-y-2 text-sm">
                                    <div className="flex justify-between gap-3">
                                        <span className="text-gray-500">当前版本</span>
                                        <span className="font-mono text-gray-800 dark:text-gray-100">v{appVersion || updateInfo?.current_version || '—'}</span>
                                    </div>
                                    <div className="flex justify-between gap-3">
                                        <span className="text-gray-500">最新版本</span>
                                        <span className="font-mono text-gray-800 dark:text-gray-100">
                                            {updateInfo?.checked ? `v${updateInfo.latest_version}` : '尚未检查'}
                                        </span>
                                    </div>
                                    {updateInfo?.update_available && (
                                        <div className="pt-2 flex items-center justify-between gap-3">
                                            <span className="text-[#007AFF] text-xs font-medium">有可用更新</span>
                                            <button
                                                type="button"
                                                onClick={openReleasePage}
                                                className="px-3 py-1.5 rounded-lg bg-[#007AFF] text-white text-xs font-semibold hover:bg-[#005ED0]"
                                            >
                                                打开发布页
                                            </button>
                                        </div>
                                    )}
                                    {!!updateInfo?.error && !updateInfo?.checked && (
                                        <div className="text-xs text-amber-600 dark:text-amber-400 break-all">离线或网络受限：{updateInfo.error}</div>
                                    )}
                                </div>
                            </div>
                        </Section>

                        <Section title="界面与其它" desc="窗口与体验行为" icon="PanelLeft">
                            <div className="space-y-3">
                                <Switch
                                    checked={Boolean(settings.remember_window_state)}
                                    onChange={(checked) => setSettings((previous) => ({ ...previous, remember_window_state: checked }))}
                                    label="记住窗口大小与位置"
                                />
                                <div className="text-xs text-gray-400 leading-relaxed">
                                    主题切换请使用标题栏旁的明暗按钮。
                                </div>
                            </div>
                        </Section>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default SettingsView;
