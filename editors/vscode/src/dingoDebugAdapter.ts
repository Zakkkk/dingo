import * as vscode from 'vscode';
import * as path from 'path';

/**
 * Dingo Debug Adapter Descriptor Factory
 *
 * This factory intercepts debug sessions of type 'dingo', builds the Dingo project,
 * and then starts a Go debug session instead. We use an inline debug adapter
 * that immediately terminates after starting the real Go session.
 */
class DingoDebugAdapterDescriptorFactory implements vscode.DebugAdapterDescriptorFactory {
    async createDebugAdapterDescriptor(
        session: vscode.DebugSession,
        executable: vscode.DebugAdapterExecutable | undefined
    ): Promise<vscode.DebugAdapterDescriptor | undefined> {
        const config = session.configuration;
        const workspaceFolder = session.workspaceFolder;
        const workspaceRoot = workspaceFolder?.uri.fsPath || '';

        // Determine the binary path based on the program
        let binaryPath: string;
        const program = config.program || '';

        if (program.endsWith('.dingo')) {
            const baseName = path.basename(program, '.dingo');
            binaryPath = path.join(workspaceRoot, baseName);
        } else {
            binaryPath = path.join(workspaceRoot, path.basename(program));
        }

        // Build the Dingo file first
        const buildSuccess = await this.buildDingo(program, workspaceRoot);
        if (!buildSuccess) {
            vscode.window.showErrorMessage('Dingo build failed. Check the terminal for errors.');
            return undefined;
        }

        // Create Go debug configuration
        const goConfig: vscode.DebugConfiguration = {
            type: 'go',
            name: config.name || 'Debug Dingo',
            request: 'launch',
            mode: 'exec',
            program: binaryPath,
            args: config.args || [],
            env: config.env || {},
            cwd: config.cwd || workspaceRoot,
        };

        // Start the Go debug session
        // Use a small delay to ensure the current session is properly set up
        setTimeout(async () => {
            await vscode.debug.startDebugging(workspaceFolder, goConfig);
        }, 100);

        // Return an inline adapter that does nothing - the Go session handles debugging
        return new vscode.DebugAdapterInlineImplementation(new DingoNoOpDebugAdapter());
    }

    private async buildDingo(program: string, cwd: string): Promise<boolean> {
        return new Promise((resolve) => {
            // Use --debug flag to emit //line directives for source mapping
            // and disable optimizations (-gcflags=-N -l) for better debugging
            const task = new vscode.Task(
                { type: 'dingo', command: 'build' },
                vscode.TaskScope.Workspace,
                'Build for Debug',
                'dingo',
                new vscode.ShellExecution(`dingo build --debug --no-mascot "${program}"`)
            );

            vscode.tasks.executeTask(task).then(
                (execution) => {
                    const disposable = vscode.tasks.onDidEndTaskProcess((e) => {
                        if (e.execution === execution) {
                            disposable.dispose();
                            resolve(e.exitCode === 0);
                        }
                    });
                },
                () => resolve(false)
            );
        });
    }
}

/**
 * A no-op debug adapter that immediately terminates.
 * Used as a placeholder while the real Go debug session starts.
 */
class DingoNoOpDebugAdapter implements vscode.DebugAdapter {
    private sendMessage: ((msg: vscode.DebugProtocolMessage) => void) | undefined;
    private onDidSendMessageEmitter = new vscode.EventEmitter<vscode.DebugProtocolMessage>();

    readonly onDidSendMessage: vscode.Event<vscode.DebugProtocolMessage> = this.onDidSendMessageEmitter.event;

    handleMessage(message: vscode.DebugProtocolMessage): void {
        const msg = message as any;

        if (msg.type === 'request') {
            if (msg.command === 'initialize') {
                // Respond to initialize
                this.sendResponse(msg, {
                    supportsConfigurationDoneRequest: false,
                });
            } else if (msg.command === 'launch' || msg.command === 'attach') {
                // Respond to launch/attach and immediately terminate
                this.sendResponse(msg, {});
                // Send terminated event
                setTimeout(() => {
                    this.onDidSendMessageEmitter.fire({
                        type: 'event',
                        event: 'terminated',
                        seq: 0,
                    } as any);
                }, 50);
            } else if (msg.command === 'disconnect') {
                this.sendResponse(msg, {});
            } else {
                // Respond to any other request
                this.sendResponse(msg, {});
            }
        }
    }

    private sendResponse(request: any, body: any): void {
        this.onDidSendMessageEmitter.fire({
            type: 'response',
            request_seq: request.seq,
            success: true,
            command: request.command,
            body: body,
            seq: 0,
        } as any);
    }

    dispose(): void {
        this.onDidSendMessageEmitter.dispose();
    }
}

/**
 * Dingo Debug Configuration Provider
 * Provides default configurations for the debugger
 */
class DingoDebugConfigProvider implements vscode.DebugConfigurationProvider {
    resolveDebugConfiguration(
        folder: vscode.WorkspaceFolder | undefined,
        config: vscode.DebugConfiguration,
        token?: vscode.CancellationToken
    ): vscode.ProviderResult<vscode.DebugConfiguration> {
        // If launch.json is missing or empty, provide default config
        if (!config.type && !config.request && !config.name) {
            const editor = vscode.window.activeTextEditor;
            if (editor && editor.document.languageId === 'dingo') {
                config.type = 'dingo';
                config.name = 'Debug Dingo';
                config.request = 'launch';
                config.program = '${file}';
            }
        }
        return config;
    }
}

/**
 * Register the Dingo debugger
 */
export function registerDingoDebugger(context: vscode.ExtensionContext) {
    // Register the debug adapter descriptor factory
    context.subscriptions.push(
        vscode.debug.registerDebugAdapterDescriptorFactory('dingo', new DingoDebugAdapterDescriptorFactory())
    );

    // Register configuration provider for defaults
    context.subscriptions.push(
        vscode.debug.registerDebugConfigurationProvider('dingo', new DingoDebugConfigProvider())
    );

    // Provide initial configurations for launch.json
    context.subscriptions.push(
        vscode.debug.registerDebugConfigurationProvider('dingo', {
            provideDebugConfigurations(folder: vscode.WorkspaceFolder | undefined): vscode.ProviderResult<vscode.DebugConfiguration[]> {
                return [
                    {
                        type: 'dingo',
                        name: 'Debug Dingo File',
                        request: 'launch',
                        program: '${file}'
                    },
                    {
                        type: 'dingo',
                        name: 'Debug Dingo Package',
                        request: 'launch',
                        program: '${workspaceFolder}'
                    }
                ];
            }
        }, vscode.DebugConfigurationProviderTriggerKind.Initial)
    );
}
