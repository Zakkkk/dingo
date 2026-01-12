import * as vscode from 'vscode';

/**
 * Error Panel for showing build/run failures with sad dingo mascot
 */
export class DingoErrorPanel {
    public static currentPanel: DingoErrorPanel | undefined;
    private readonly panel: vscode.WebviewPanel;
    private readonly extensionUri: vscode.Uri;
    private disposables: vscode.Disposable[] = [];

    private constructor(panel: vscode.WebviewPanel, extensionUri: vscode.Uri) {
        this.panel = panel;
        this.extensionUri = extensionUri;

        // Handle panel disposal
        this.panel.onDidDispose(() => this.dispose(), null, this.disposables);

        // Handle messages from webview
        this.panel.webview.onDidReceiveMessage(
            message => {
                switch (message.command) {
                    case 'retry':
                        this.handleRetry(message.action);
                        break;
                    case 'close':
                        this.panel.dispose();
                        break;
                }
            },
            null,
            this.disposables
        );
    }

    /**
     * Show error panel with sad dingo and error details
     */
    public static show(
        extensionUri: vscode.Uri,
        errorType: 'build' | 'run' | 'panic',
        errorMessage: string,
        filePath?: string
    ) {
        const column = vscode.window.activeTextEditor
            ? vscode.window.activeTextEditor.viewColumn
            : undefined;

        // Dispose existing panel if any
        if (DingoErrorPanel.currentPanel) {
            DingoErrorPanel.currentPanel.panel.dispose();
        }

        // Create new panel
        const panel = vscode.window.createWebviewPanel(
            'dingoError',
            'Dingo Error',
            column || vscode.ViewColumn.One,
            {
                enableScripts: true,
                localResourceRoots: [vscode.Uri.joinPath(extensionUri, 'icons')]
            }
        );

        DingoErrorPanel.currentPanel = new DingoErrorPanel(panel, extensionUri);
        DingoErrorPanel.currentPanel.update(errorType, errorMessage, filePath);
    }

    private update(errorType: 'build' | 'run' | 'panic', errorMessage: string, filePath?: string) {
        this.panel.title = errorType === 'panic' ? 'Dingo Panic!' : `Dingo ${errorType.charAt(0).toUpperCase() + errorType.slice(1)} Failed`;
        this.panel.webview.html = this.getHtml(errorType, errorMessage, filePath);
    }

    private getHtml(errorType: 'build' | 'run' | 'panic', errorMessage: string, filePath?: string): string {
        const title = errorType === 'panic'
            ? 'Panic!'
            : errorType === 'build'
                ? 'Build Failed'
                : 'Run Failed';

        const subtitle = errorType === 'panic'
            ? 'Your program crashed with a panic'
            : errorType === 'build'
                ? 'Failed to compile the project'
                : 'Program exited with an error';

        // Escape HTML in error message
        const escapedError = errorMessage
            .replace(/&/g, '&amp;')
            .replace(/</g, '&lt;')
            .replace(/>/g, '&gt;')
            .replace(/"/g, '&quot;')
            .replace(/'/g, '&#039;');

        // Highlight panic message and stack trace
        const formattedError = escapedError
            .replace(/^(panic:.*)$/gm, '<span class="panic-line">$1</span>')
            .replace(/(\s+)(\/[^\s]+\.go:\d+)/g, '$1<span class="file-path">$2</span>')
            .replace(/(goroutine \d+ \[.+\]:)/g, '<span class="goroutine">$1</span>');

        return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>${title}</title>
    <style>
        body {
            font-family: var(--vscode-font-family);
            padding: 20px;
            color: var(--vscode-foreground);
            background-color: var(--vscode-editor-background);
        }
        .container {
            max-width: 800px;
            margin: 0 auto;
        }
        .header {
            display: flex;
            align-items: center;
            gap: 20px;
            margin-bottom: 20px;
        }
        .mascot {
            font-size: 80px;
            line-height: 1;
        }
        .title-section h1 {
            margin: 0;
            color: #e74c3c;
            font-size: 28px;
        }
        .title-section p {
            margin: 5px 0 0 0;
            color: var(--vscode-descriptionForeground);
        }
        .error-box {
            background-color: var(--vscode-inputValidation-errorBackground, rgba(231, 76, 60, 0.1));
            border: 1px solid var(--vscode-inputValidation-errorBorder, #e74c3c);
            border-radius: 4px;
            padding: 15px;
            margin: 20px 0;
            overflow-x: auto;
        }
        .error-content {
            font-family: var(--vscode-editor-font-family, monospace);
            font-size: 13px;
            white-space: pre-wrap;
            word-break: break-word;
            line-height: 1.5;
        }
        .panic-line {
            color: #e74c3c;
            font-weight: bold;
        }
        .file-path {
            color: var(--vscode-textLink-foreground);
            text-decoration: underline;
            cursor: pointer;
        }
        .goroutine {
            color: #9b59b6;
        }
        .actions {
            display: flex;
            gap: 10px;
            margin-top: 20px;
        }
        button {
            padding: 8px 16px;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 14px;
        }
        .retry-btn {
            background-color: var(--vscode-button-background);
            color: var(--vscode-button-foreground);
        }
        .retry-btn:hover {
            background-color: var(--vscode-button-hoverBackground);
        }
        .close-btn {
            background-color: var(--vscode-button-secondaryBackground);
            color: var(--vscode-button-secondaryForeground);
        }
        .close-btn:hover {
            background-color: var(--vscode-button-secondaryHoverBackground);
        }
        .file-info {
            color: var(--vscode-descriptionForeground);
            font-size: 12px;
            margin-bottom: 10px;
        }
        .sad-dingo {
            color: #e74c3c;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <div class="mascot sad-dingo">
                <pre style="font-size: 14px; line-height: 1.2;">
    /\\___/\\
   (  x x  )
   (  ＞＜ )
    /|   |\\
   (_|   |_)
                </pre>
            </div>
            <div class="title-section">
                <h1>${title}</h1>
                <p>${subtitle}</p>
            </div>
        </div>

        ${filePath ? `<div class="file-info">File: ${filePath}</div>` : ''}

        <div class="error-box">
            <div class="error-content">${formattedError}</div>
        </div>

        <div class="actions">
            <button class="retry-btn" onclick="retry()">
                ${errorType === 'build' ? '🔨 Retry Build' : '▶️ Run Again'}
            </button>
            <button class="close-btn" onclick="close()">Close</button>
        </div>
    </div>

    <script>
        const vscode = acquireVsCodeApi();

        function retry() {
            vscode.postMessage({ command: 'retry', action: '${errorType}' });
        }

        function close() {
            vscode.postMessage({ command: 'close' });
        }
    </script>
</body>
</html>`;
    }

    private handleRetry(action: string) {
        this.panel.dispose();

        if (action === 'build') {
            vscode.commands.executeCommand('dingo.buildCurrentFile');
        } else {
            vscode.commands.executeCommand('dingo.runCurrentFile');
        }
    }

    public dispose() {
        DingoErrorPanel.currentPanel = undefined;
        this.panel.dispose();
        while (this.disposables.length) {
            const disposable = this.disposables.pop();
            if (disposable) {
                disposable.dispose();
            }
        }
    }
}

/**
 * Parse terminal output for errors and show error panel if detected
 */
export function parseAndShowError(
    extensionUri: vscode.Uri,
    output: string,
    errorType: 'build' | 'run',
    filePath?: string
): boolean {
    // Detect panic
    if (output.includes('panic:')) {
        DingoErrorPanel.show(extensionUri, 'panic', output, filePath);
        return true;
    }

    // Detect build errors
    if (errorType === 'build' && (
        output.includes('error:') ||
        output.includes('cannot find') ||
        output.includes('undefined:') ||
        output.includes('syntax error')
    )) {
        DingoErrorPanel.show(extensionUri, 'build', output, filePath);
        return true;
    }

    // Detect run errors (non-zero exit)
    if (errorType === 'run' && output.includes('exit status')) {
        DingoErrorPanel.show(extensionUri, 'run', output, filePath);
        return true;
    }

    return false;
}
