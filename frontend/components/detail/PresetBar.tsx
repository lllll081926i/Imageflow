import React from 'react';
import { CustomSelect } from '../Controls';
import { usePagePresets } from './usePagePresets';

type PresetBarProps = {
    scope: string;
    buildPayload: () => Record<string, unknown>;
    applyPayload: (payload: Record<string, unknown>) => void;
};

export const PresetBar: React.FC<PresetBarProps> = ({ scope, buildPayload, applyPayload }) => {
    const { presets, activeId, setActiveId, save, remove, apply } = usePagePresets(scope);
    const [nameDraft, setNameDraft] = React.useState('');

    return (
        <div className="space-y-2">
            <div className="flex items-center gap-2">
                <div className="flex-1 min-w-0">
                    <CustomSelect
                        options={[
                            { label: '选择预设', value: '' },
                            ...presets.map((p) => ({ label: p.name, value: p.id })),
                        ]}
                        value={activeId}
                        onChange={(id) => {
                            setActiveId(id);
                            if (!id) return;
                            const payload = apply(id);
                            if (payload) applyPayload(payload);
                        }}
                    />
                </div>
                {activeId && (
                    <button
                        type="button"
                        onClick={() => remove(activeId)}
                        className="spring-tap-sm shrink-0 px-2.5 py-2 rounded-xl border border-red-200 dark:border-red-500/30 text-xs font-medium text-red-500 hover:bg-red-50 dark:hover:bg-red-500/10 transition-colors"
                        title="删除当前预设"
                    >
                        删除
                    </button>
                )}
            </div>
            <div className="flex items-center gap-2">
                <input
                    type="text"
                    value={nameDraft}
                    onChange={(e) => setNameDraft(e.target.value)}
                    placeholder="预设名称"
                    className="flex-1 min-w-0 px-3 py-2 rounded-xl bg-gray-50 dark:bg-white/5 border border-gray-200 dark:border-white/10 text-xs outline-none dark:text-white focus:border-[#007AFF] transition-colors"
                />
                <button
                    type="button"
                    onClick={() => {
                        save(nameDraft, buildPayload());
                        setNameDraft('');
                    }}
                    className="spring-tap-sm shrink-0 px-3 py-2 rounded-xl border border-gray-200 dark:border-white/10 text-xs font-medium text-gray-700 dark:text-gray-200 hover:bg-gray-50 dark:hover:bg-white/10 transition-colors"
                >
                    保存当前
                </button>
            </div>
        </div>
    );
};
