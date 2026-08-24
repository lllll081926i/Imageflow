import React, { useCallback, useEffect, useMemo, useState } from 'react';
import Icon from './Icon';
import { SegmentedControl, Switch } from './Controls';
import { showToast } from './Toast';
import type { Theme } from '../types';
import {
    DEFAULT_APP_SETTINGS,
    applyAppearance,
    getAppBindings,
    loadAppSettings,
    saveAppSettings,
    type AppSettingsSnapshot,
} from '../types/app-api';

const clamp = (value: number, min: number, max: number) => Math.max(min, Math.min(max, value));

const CONFLICT_STRATEGY_LABELS: Record<string, string> = {
    rename: '自动重命名',
    overwrite: '直接覆盖',
    skip: '跳过同名',
};

type UpdateCheckResult = {
    success?: boolean;
    current_version?: string;
    latest_version?: string;
    update_available?: boolean;
    release_url?: string;
    release_notes?: string;
    checked?: boolean;
    error?: string;
    distribution_kind?: 'installed' | 'portable' | 'development';
    platform?: string;
    update_action?: 'install' | 'open_release';
    installer_available?: boolean;
    proxy_used?: boolean;
    cached?: boolean;
};

const Section: React.FC<{
    title: string;
    icon: string;
    children: React.ReactNode;
    action?: React.ReactNode;
}> = ({ title, icon, children, action }) => (
    <section className="bg-white dark:bg-[#2C2C2E] rounded-3xl border border-gray-200 dark:border-white/10 shadow-sm p-4 sm:p-6 min-w-0">
        <div className="flex items-start justify-between gap-4 mb-5">
            <div className="flex items-start gap-3 min-w-0">
                <div className="w-10 h-10 rounded-2xl bg-[#007AFF]/10 text-[#007AFF] flex items-center justify-center shrink-0">
                    <Icon name={icon as any} size={18} />
                </div>
                <div className="min-w-0">
                    <div className="text-base font-semibold text-gray-900 dark:text-white">{title}</div>
                </div>
            </div>
            {action}
        </div>
        {children}
    </section>
);

type SettingsViewProps = {
    themePreference: Theme;
    onThemeChange: (theme: Theme) => void;
};

const SettingsView: React.FC<SettingsViewProps> = ({ themePreference, onThemeChange }) => {
    const [settings, setSettings] = useState<AppSettingsSnapshot>({ ...DEFAULT_APP_SETTINGS });
    const [saving, setSaving] = useState(false);
    const [loading, setLoading] = useState(true);
    const [message, setMessage] = useState('');
    const [appVersion, setAppVersion] = useState('');
    const [updateBusy, setUpdateBusy] = useState(false);
    const [installBusy, setInstallBusy] = useState(false);
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
            const res = (await app.CheckForUpdates({ timeout_seconds: 8, force: !silent })) as UpdateCheckResult;
            if (!silent || res?.checked || !res?.error) setUpdateInfo(res || null);
            if (res?.current_version) setAppVersion(String(res.current_version));
            if (!silent) {
                if (res?.update_available) {
                    setMessage(`发现新版本 ${res.latest_version}`);
                } else if (res?.error) {
                    setMessage(`检查更新失败：${res.error}`);
                } else {
                    setMessage('');
                }
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
                const loaded = await loadAppSettings({ throwOnError: true, force: true });
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
            applyAppearance(saved);
            showToast('已保存', 'success');
            setMessage('');
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

    const openReleasePage = async () => {
        const url = updateInfo?.release_url || 'https://github.com/lllll081926i/Imageflow/releases';
        try {
            const app = getAppBindings() as any;
            if (app?.OpenReleasePage) {
                const result = await app.OpenReleasePage({ url });
                if (result?.success === false) throw new Error(result?.error || '系统浏览器未能打开发布页');
                return;
            }
            window.open(url, '_blank', 'noopener,noreferrer');
        } catch (error: any) {
            setMessage(error?.message ? `无法打开发布页：${error.message}` : '无法打开发布页');
        }
    };

    const handleUpdate = async () => {
        if (!updateInfo?.update_available) return;
        if (updateInfo.update_action !== 'install') {
            await openReleasePage();
            return;
        }

        const version = String(updateInfo.latest_version || '').trim();
        const confirmed = window.confirm(
            `确认更新到 ImageFlow v${version}？\n\n软件将使用系统或环境中已配置的本地代理下载安装包，完成校验后启动安装程序。`,
        );
        if (!confirmed) return;

        const app = getAppBindings() as any;
        if (!app?.DownloadAndInstallUpdate) {
            setMessage('当前桌面宿主不支持自动安装，请前往发布页更新');
            await openReleasePage();
            return;
        }

        setInstallBusy(true);
        setMessage(`正在安全下载并校验 v${version}…`);
        try {
            const result = await app.DownloadAndInstallUpdate({ expected_version: version, timeout_seconds: 180 });
            if (!result?.success) throw new Error(result?.error || '更新失败');
            if (result.action === 'installer_started') {
                setMessage('安装程序已启动，请按系统提示完成更新');
                showToast('安装程序已启动', 'success');
            } else if (result.action === 'release_page') {
                setMessage('已在系统浏览器中打开发布页');
            } else {
                setMessage(result?.message || '当前已是最新版本');
            }
        } catch (error: any) {
            setMessage(error?.message ? `更新失败：${error.message}` : '更新失败');
            showToast('更新失败，请重试或前往发布页', 'error');
        } finally {
            setInstallBusy(false);
        }
    };

    return (
        <div className="h-full flex flex-col min-h-0">
            <div className="flex items-start justify-between gap-4 mb-6 flex-wrap">
                <div className="flex items-center gap-3 min-w-0">
                    <div className="w-11 h-11 rounded-2xl bg-white dark:bg-[#2C2C2E] border border-gray-200 dark:border-white/10 shadow-sm flex items-center justify-center">
                        <Icon name="Settings" size={20} className="text-[#007AFF]" />
                    </div>
                    <div>
                        <div className="text-xl font-semibold text-gray-900 dark:text-white">全局设置</div>
                        {appVersion && <div className="text-sm text-gray-500 dark:text-gray-400 mt-1">v{appVersion}</div>}
                    </div>
                </div>
                <div className="flex items-center gap-3 flex-wrap justify-end min-w-0">
                    <div className="text-xs text-gray-500 dark:text-gray-400 min-w-0 max-w-full text-right">
                        {loading
                            ? '读取中...'
                            : (message ? <span className={message.includes('失败') ? 'text-red-500' : 'text-[#007AFF]'}>{message}</span> : null)}
                    </div>
                    <button
                        type="button"
                        disabled={saving || loading}
                        onClick={() => void handleSave({ ...settings, max_concurrency: maxConcurrency })}
                        className={`spring-tap-sm px-4 py-2.5 rounded-xl text-sm font-semibold transition-all duration-300 ${(saving || loading) ? 'bg-gray-200 dark:bg-white/10 text-gray-500 dark:text-gray-400 cursor-not-allowed' : 'bg-[#007AFF] text-white hover:bg-[#005ED0]'}`}
                    >
                        {saving ? '保存中...' : '保存全部'}
                    </button>
                </div>
            </div>

            <div className="flex-1 min-h-0 min-w-0 overflow-y-auto pr-1">
                <div className="imageflow-settings-grid grid gap-6">
                    <div className="space-y-6">
                        <Section
                            title="常用路径"
                            icon="Upload"
                        >
                            <div className="space-y-5">
                                <div className="space-y-2">
                                    <label className="text-sm font-medium text-gray-700 dark:text-gray-300">默认输出目录</label>
                                    <div className="flex items-center gap-2">
                                        <input
                                            type="text"
                                            value={settings.default_output_dir}
                                            onChange={(event) => setSettings((previous) => ({ ...previous, default_output_dir: event.target.value }))}
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
                            </div>
                        </Section>

                        <Section title="输出规则" icon="FileType">
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
                                    <div className="space-y-2">
                                        <div className="text-sm text-gray-500 dark:text-gray-400">重名文件</div>
                                        <SegmentedControl
                                            options={['自动重命名', '直接覆盖', '跳过同名']}
                                            value={CONFLICT_STRATEGY_LABELS[settings.conflict_strategy] ?? '自动重命名'}
                                            onChange={(label) => {
                                                const entry = Object.entries(CONFLICT_STRATEGY_LABELS).find(([, text]) => text === label);
                                                if (entry) {
                                                    setSettings((previous) => ({ ...previous, conflict_strategy: entry[0] as typeof previous.conflict_strategy }));
                                                }
                                            }}
                                        />
                                    </div>
                                    <Switch
                                        checked={Boolean(settings.open_output_folder_on_complete)}
                                        onChange={(checked) => setSettings((previous) => ({ ...previous, open_output_folder_on_complete: checked }))}
                                        label="处理完成后打开输出目录"
                                    />
                                </div>
                            </div>
                        </Section>

                        <Section title="外观" icon="Sun">
                            <div className="rounded-2xl bg-gray-50 dark:bg-white/5 border border-gray-200/80 dark:border-white/10 p-4 space-y-3">
                                <Switch
                                    checked={Boolean(settings.ui_transparency)}
                                    onChange={(checked) => {
                                        setSettings((previous) => ({ ...previous, ui_transparency: checked }));
                                        applyAppearance({ ...settings, ui_transparency: checked });
                                    }}
                                    label="透明效果"
                                />
                                <Switch
                                    checked={Boolean(settings.ui_reduced_motion)}
                                    onChange={(checked) => {
                                        setSettings((previous) => ({ ...previous, ui_reduced_motion: checked }));
                                        applyAppearance({ ...settings, ui_reduced_motion: checked });
                                    }}
                                    label="减弱动态效果"
                                />
                            </div>
                        </Section>
                    </div>

                    <div className="space-y-6">
                        <Section title="处理性能" icon="Layers">
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
                        </Section>

                        <Section
                            title="软件更新"
                            icon="ArrowRight"
                            action={(
                                <button
                                    type="button"
                                    disabled={updateBusy || installBusy || loading}
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
                                                disabled={installBusy}
                                                onClick={() => void handleUpdate()}
                                                className="spring-tap-sm px-3 py-1.5 rounded-lg bg-[#007AFF] text-white text-xs font-semibold hover:bg-[#005ED0] transition-all duration-300 disabled:opacity-60 disabled:cursor-wait"
                                            >
                                                {installBusy
                                                    ? '正在准备更新…'
                                                    : updateInfo.update_action === 'install'
                                                        ? '下载并安装'
                                                        : '打开发布页'}
                                            </button>
                                        </div>
                                    )}
                                    {!!updateInfo?.error && !updateInfo?.checked && (
                                        <div className="text-xs text-amber-600 dark:text-amber-400 break-all">离线或网络受限：{updateInfo.error}</div>
                                    )}
                                </div>
                            </div>
                        </Section>

                        <Section title="界面与其它" icon="PanelLeft">
                            <div className="space-y-3">
                                <div className="space-y-2">
                                    <div className="text-sm font-medium text-gray-700 dark:text-gray-300">主题</div>
                                    <SegmentedControl
                                        options={['浅色', '深色', '跟随系统']}
                                        value={themePreference === 'light' ? '浅色' : themePreference === 'dark' ? '深色' : '跟随系统'}
                                        onChange={(value) => onThemeChange(value === '浅色' ? 'light' : value === '深色' ? 'dark' : 'system')}
                                    />
                                </div>
                                    <Switch
                                        checked={Boolean(settings.remember_window_state)}
                                        onChange={(checked) => setSettings((previous) => ({ ...previous, remember_window_state: checked }))}
                                        label="记住窗口大小与位置"
                                    />
                                    <Switch
                                        checked={Boolean(settings.background_running)}
                                        onChange={(checked) => setSettings((previous) => ({ ...previous, background_running: checked }))}
                                        label="后台运行"
                                    />
                            </div>
                        </Section>
                    </div>
                </div>
            </div>
        </div>
    );
};

export default SettingsView;
