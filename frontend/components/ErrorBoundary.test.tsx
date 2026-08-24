// @vitest-environment jsdom

import React from 'react';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import ErrorBoundary from './ErrorBoundary';
import DetailView from './DetailView';
import SettingsView from './SettingsView';
import { FEATURES } from '../constants';

function Boom(): React.ReactElement {
    throw new Error('render exploded');
}

function Ok({ label }: { label: string }): React.ReactElement {
    return <div data-testid="ok">{label}</div>;
}

describe('ErrorBoundary', () => {
    beforeAll(() => {
        // React 19 testing in jsdom.
        (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
    });

    afterEach(() => {
        document.body.innerHTML = '';
        if (typeof localStorage?.clear === 'function') localStorage.clear();
        delete (window as { runtime?: unknown }).runtime;
        delete (window as { pywebview?: unknown }).pywebview;
        vi.restoreAllMocks();
    });

    it('renders children when no error occurs', async () => {
        const host = document.createElement('div');
        document.body.appendChild(host);
        const root = createRoot(host);

        await act(async () => {
            root.render(
                <ErrorBoundary>
                    <Ok label="healthy" />
                </ErrorBoundary>,
            );
        });

        expect(host.textContent).toContain('healthy');
        await act(async () => {
            root.unmount();
        });
    });

    it('renders fallback UI when child throws during render', async () => {
        const host = document.createElement('div');
        document.body.appendChild(host);
        const root = createRoot(host);
        const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);

        await act(async () => {
            root.render(
                <ErrorBoundary title="页面渲染出错">
                    <Boom />
                </ErrorBoundary>,
            );
        });

        expect(host.textContent).toContain('页面渲染出错');
        expect(host.textContent).toContain('render exploded');
        expect(host.textContent).toContain('重试');
        errorSpy.mockRestore();

        await act(async () => {
            root.unmount();
        });
    });

    it('reset button clears error state and re-renders children', async () => {
        const host = document.createElement('div');
        document.body.appendChild(host);
        const root = createRoot(host);
        const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined);

        let shouldThrow = true;
        function MaybeBoom(): React.ReactElement {
            if (shouldThrow) {
                throw new Error('temporary boom');
            }
            return <div>recovered</div>;
        }

        await act(async () => {
            root.render(
                <ErrorBoundary title="可恢复错误">
                    <MaybeBoom />
                </ErrorBoundary>,
            );
        });
        expect(host.textContent).toContain('temporary boom');

        shouldThrow = false;
        const button = host.querySelector('button');
        expect(button).toBeTruthy();
        await act(async () => {
            button?.dispatchEvent(new MouseEvent('click', { bubbles: true }));
        });
        expect(host.textContent).toContain('recovered');
        errorSpy.mockRestore();

        await act(async () => {
            root.unmount();
        });
    });

    it('11 个工具页与设置页都能独立渲染并提供可操作入口', async () => {
        const host = document.createElement('div');
        document.body.appendChild(host);
        const root = createRoot(host);

        for (const feature of FEATURES) {
            await act(async () => {
                root.render(
                    <ErrorBoundary title="页面渲染出错">
                        <DetailView key={feature.id} id={feature.id} isActive={false} />
                    </ErrorBoundary>,
                );
                await Promise.resolve();
                await Promise.resolve();
            });

            expect(host.textContent, `${feature.title} 不应进入错误边界`).not.toContain('页面渲染出错');
            expect(
                host.querySelector(`[data-feature-id="${feature.id}"][aria-label="${feature.title}"]`),
                `${feature.title} 应暴露正确的页面区域语义`,
            ).toBeTruthy();
            if (feature.id === 'richtext') {
                expect(
                    host.querySelector('textarea, [contenteditable="true"]'),
                    `${feature.title} 应提供文字编辑入口`,
                ).toBeTruthy();
            } else {
                expect(
                    host.querySelector('[style*="--if-drop-target"]'),
                    `${feature.title} 应提供文件拖放入口`,
                ).toBeTruthy();
            }
            expect(host.querySelectorAll('button').length, `${feature.title} 应提供操作控件`).toBeGreaterThan(0);
        }

        await act(async () => {
            root.render(
                <ErrorBoundary title="页面渲染出错">
                    <SettingsView themePreference="system" onThemeChange={vi.fn()} />
                </ErrorBoundary>,
            );
            await Promise.resolve();
            await Promise.resolve();
        });
        expect(host.textContent).toContain('全局设置');
        expect(host.textContent).not.toContain('页面渲染出错');
        expect(host.querySelectorAll('button').length).toBeGreaterThan(0);

        await act(async () => {
            root.unmount();
        });
    });
});
