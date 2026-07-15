import React, { Component, type ErrorInfo, type ReactNode } from 'react';
import Icon from './Icon';

type ErrorBoundaryProps = {
    children: ReactNode;
    title?: string;
    onReset?: () => void;
};

type ErrorBoundaryState = {
    error: Error | null;
};

class ErrorBoundary extends Component<ErrorBoundaryProps, ErrorBoundaryState> {
    constructor(props: ErrorBoundaryProps) {
        super(props);
        this.state = { error: null };
    }

    static getDerivedStateFromError(error: Error): ErrorBoundaryState {
        return { error };
    }

    componentDidCatch(error: Error, info: ErrorInfo) {
        console.error('UI render error:', error, info.componentStack);
    }

    handleReset = () => {
        this.setState({ error: null });
        if (this.props.onReset) {
            this.props.onReset();
        }
    };

    render() {
        if (!this.state.error) {
            return this.props.children;
        }

        const title = this.props.title || '界面渲染出错';
        const message = this.state.error.message || '未知错误';

        return (
            <div className="h-full w-full flex items-center justify-center p-6">
                <div className="max-w-lg w-full rounded-2xl border border-red-200/80 bg-white/90 dark:bg-[#1C1C1E]/90 dark:border-red-900/50 shadow-xl p-6 space-y-4">
                    <div className="flex items-center gap-3 text-red-600 dark:text-red-400">
                        <Icon name="AlertTriangle" size={22} />
                        <h2 className="text-lg font-semibold">{title}</h2>
                    </div>
                    <p className="text-sm text-gray-600 dark:text-gray-300 break-words">{message}</p>
                    <button
                        type="button"
                        onClick={this.handleReset}
                        className="inline-flex items-center justify-center rounded-xl bg-[#007AFF] hover:bg-[#0066DD] text-white px-4 py-2 text-sm font-medium"
                    >
                        重试
                    </button>
                </div>
            </div>
        );
    }
}

export default ErrorBoundary;
