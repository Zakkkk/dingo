import * as vscode from 'vscode';
import * as path from 'path';
import {
    LanguageClient,
    LanguageClientOptions,
    ServerOptions,
    TransportKind
} from 'vscode-languageclient/node';

let client: LanguageClient | null = null;

export async function activateLSPClient(context: vscode.ExtensionContext): Promise<void> {
    // Guard against double initialization (can happen with multiple activation events)
    if (client) {
        console.log('Dingo LSP client already running, skipping duplicate activation');
        return;
    }

    const config = vscode.workspace.getConfiguration('dingo');

    // Check if LSP is enabled (could add opt-out setting later)
    const lspPath = config.get<string>('lsp.path', 'dingo-lsp');
    const logLevel = config.get<string>('lsp.logLevel', 'info');
    const transpileOnSave = config.get<boolean>('transpileOnSave', true);
    const sqliteLogging = config.get<boolean>('lsp.sqliteLogging', false);
    const sqliteLogPath = config.get<string>('lsp.sqliteLogPath', '');

    // Build environment variables
    const env: NodeJS.ProcessEnv = {
        ...process.env,
        DINGO_LSP_LOG: logLevel,
        DINGO_AUTO_TRANSPILE: transpileOnSave.toString(),
    };

    // Add SQLite logging path if enabled
    if (sqliteLogging) {
        // Use provided path or default to temp directory
        const os = require('os');
        const dbPath = sqliteLogPath || path.join(os.tmpdir(), 'dingo-lsp.db');
        env.DINGO_LSP_SQLITE = dbPath;
        console.log(`Dingo LSP SQLite logging to: ${dbPath}`);
    }

    // Server options - start dingo-lsp binary
    const serverOptions: ServerOptions = {
        command: lspPath,
        args: [],
        transport: TransportKind.stdio,
        options: { env }
    };

    // Client options - document selector and synchronization
    const clientOptions: LanguageClientOptions = {
        documentSelector: [
            { scheme: 'file', language: 'dingo' }
        ],
        synchronize: {
            // Notify server of .dingo and .go.map file changes
            fileEvents: vscode.workspace.createFileSystemWatcher('**/*.{dingo,go.map}')
        },
        outputChannelName: 'Dingo Language Server',
        // Show error notifications and restart on errors
        errorHandler: {
            error: () => ({ action: 1 }), // Restart on error (was: 2 Continue)
            closed: () => ({ action: 1 })  // Restart on close
        },
        // Handle initialization failures
        initializationFailedHandler: (error) => {
            vscode.window.showErrorMessage(
                `Dingo LSP initialization failed: ${error.message}`,
                'View Output'
            ).then(selection => {
                if (selection === 'View Output') {
                    client?.outputChannel.show();
                }
            });
            return false; // Don't retry immediately
        },
        // Middleware to deduplicate results (prevents duplicates from multiple providers)
        middleware: {
            provideHover: (document, position, token, next) => {
                const requestId = `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
                console.log(`[Dingo Hover ${requestId}] START: ${document.uri.toString()}:${position.line}:${position.character}`);
                return Promise.resolve(next(document, position, token)).then(result => {
                    console.log(`[Dingo Hover ${requestId}] RESULT: ${result ? 'present' : 'null'}`);
                    if (result && (result as any).contents) {
                        const contents = (result as any).contents;
                        console.log(`[Dingo Hover ${requestId}] Content type: ${typeof contents}, kind: ${contents.kind || 'N/A'}`);
                        if (contents.value) {
                            console.log(`[Dingo Hover ${requestId}] Content preview: ${contents.value.substring(0, 100)}...`);
                        }
                    }
                    return result;
                });
            },
            provideDefinition: (document, position, token, next) => {
                return Promise.resolve(next(document, position, token)).then(result => {
                    if (!result || !Array.isArray(result)) {
                        return result;
                    }
                    // Deduplicate locations by URI + range
                    const seen = new Set<string>();
                    const filtered = (result as any[]).filter((loc: any) => {
                        const uri = loc.uri?.toString() || loc.targetUri?.toString() || '';
                        const range = loc.range || loc.targetRange;
                        const key = `${uri}:${range?.start?.line}:${range?.start?.character}`;
                        if (seen.has(key)) {
                            console.log('Dingo definition middleware: filtered duplicate', key);
                            return false;
                        }
                        seen.add(key);
                        return true;
                    });
                    console.log(`Dingo definition middleware: ${result.length} -> ${filtered.length} results`);
                    return filtered as any;
                });
            }
        }
    };

    // Create and start the language client
    client = new LanguageClient(
        'dingo-lsp',
        'Dingo Language Server',
        serverOptions,
        clientOptions
    );

    try {
        await client.start();
        console.log('Dingo LSP client started successfully');

        // Show notification if gopls is not installed
        client.onNotification('window/showMessage', (params: any) => {
            if (params.message.includes('gopls not found')) {
                vscode.window.showErrorMessage(
                    params.message,
                    'Install gopls'
                ).then(selection => {
                    if (selection === 'Install gopls') {
                        vscode.env.openExternal(vscode.Uri.parse('https://github.com/golang/tools/tree/master/gopls#installation'));
                    }
                });
            }
        });

    } catch (error) {
        console.error('Failed to start Dingo LSP client:', error);

        // Show helpful error message
        if ((error as Error).message.includes('ENOENT') || (error as Error).message.includes('not found')) {
            vscode.window.showErrorMessage(
                'dingo-lsp binary not found. Please ensure dingo is installed and dingo-lsp is in your PATH.',
                'Install Dingo'
            ).then(selection => {
                if (selection === 'Install Dingo') {
                    vscode.env.openExternal(vscode.Uri.parse('https://dingolang.com/docs/installation'));
                }
            });
        } else {
            vscode.window.showErrorMessage(`Failed to start Dingo LSP: ${(error as Error).message}`);
        }
    }

    // Register commands only if not already registered (prevents double-registration errors)
    const commands = await vscode.commands.getCommands(true);

    if (!commands.includes('dingo.transpileCurrentFile')) {
        context.subscriptions.push(
            vscode.commands.registerCommand('dingo.transpileCurrentFile', async () => {
                const editor = vscode.window.activeTextEditor;
                if (!editor || editor.document.languageId !== 'dingo') {
                    vscode.window.showErrorMessage('Not a Dingo file');
                    return;
                }

                const filePath = editor.document.uri.fsPath;
                const terminal = vscode.window.createTerminal('Dingo Transpile');
                terminal.sendText(`dingo build ${filePath}`);
                terminal.show();
            })
        );
    }

    if (!commands.includes('dingo.transpileWorkspace')) {
        context.subscriptions.push(
            vscode.commands.registerCommand('dingo.transpileWorkspace', async () => {
                const terminal = vscode.window.createTerminal('Dingo Transpile');
                terminal.sendText('dingo build ./...');
                terminal.show();
            })
        );
    }

    if (!commands.includes('dingo.restartLSP')) {
        context.subscriptions.push(
            vscode.commands.registerCommand('dingo.restartLSP', async () => {
                if (client) {
                    await client.stop();
                    await client.start();
                    vscode.window.showInformationMessage('Dingo LSP restarted');
                } else {
                    vscode.window.showWarningMessage('Dingo LSP is not running');
                }
            })
        );
    }
}

export async function deactivateLSPClient(): Promise<void> {
    if (client) {
        await client.stop();
        client = null;
    }
}

export function getLSPClient(): LanguageClient | null {
    return client;
}
