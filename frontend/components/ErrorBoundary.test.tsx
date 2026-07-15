// @vitest-environment jsdom

import React from 'react';
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import ErrorBoundary from './ErrorBoundary';

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
});
