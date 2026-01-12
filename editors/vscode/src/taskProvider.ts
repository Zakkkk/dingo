import * as vscode from 'vscode';
import * as path from 'path';
import * as cp from 'child_process';
import { DingoErrorPanel } from './errorPanel';

/**
 * Dingo Task Provider
 * Provides build and run tasks for .dingo files
 */
export class DingoTaskProvider implements vscode.TaskProvider {
    static DingoType = 'dingo';
    private tasks: vscode.Task[] | undefined;

    constructor(private workspaceRoot: string | undefined) {}

    public async provideTasks(): Promise<vscode.Task[]> {
        return this.getTasks();
    }

    public resolveTask(task: vscode.Task): vscode.Task | undefined {
        const definition = task.definition as DingoTaskDefinition;
        if (definition.type === DingoTaskProvider.DingoType) {
            return this.getTask(definition);
        }
        return undefined;
    }

    private getTasks(): vscode.Task[] {
        if (this.tasks !== undefined) {
            return this.tasks;
        }

        this.tasks = [];

        // Build current file task
        this.tasks.push(this.createBuildTask('Build Current File', '${file}', 'build'));

        // Build workspace task
        this.tasks.push(this.createBuildTask('Build Workspace', '.', 'build'));

        // Run current file task
        this.tasks.push(this.createRunTask('Run Current File', '${file}'));

        return this.tasks;
    }

    private createBuildTask(name: string, target: string, command: string): vscode.Task {
        const definition: DingoTaskDefinition = {
            type: DingoTaskProvider.DingoType,
            command: command,
            target: target
        };

        const task = new vscode.Task(
            definition,
            vscode.TaskScope.Workspace,
            name,
            'dingo',
            new vscode.ShellExecution(`dingo ${command} ${target}`),
            '$dingo' // Problem matcher
        );

        task.group = vscode.TaskGroup.Build;
        task.presentationOptions = {
            reveal: vscode.TaskRevealKind.Always,
            panel: vscode.TaskPanelKind.Shared
        };

        return task;
    }

    private createRunTask(name: string, target: string): vscode.Task {
        const definition: DingoTaskDefinition = {
            type: DingoTaskProvider.DingoType,
            command: 'run',
            target: target
        };

        const task = new vscode.Task(
            definition,
            vscode.TaskScope.Workspace,
            name,
            'dingo',
            new vscode.ShellExecution(`dingo run ${target}`),
            '$dingo'
        );

        task.group = vscode.TaskGroup.Build;
        task.presentationOptions = {
            reveal: vscode.TaskRevealKind.Always,
            panel: vscode.TaskPanelKind.Shared,
            focus: true
        };

        return task;
    }

    private getTask(definition: DingoTaskDefinition): vscode.Task {
        const command = definition.command || 'build';
        const target = definition.target || '.';
        const args = definition.args || [];

        const argsStr = args.length > 0 ? ' ' + args.join(' ') : '';
        const shellCmd = `dingo ${command} ${target}${argsStr}`;

        return new vscode.Task(
            definition,
            vscode.TaskScope.Workspace,
            `${command} ${target}`,
            'dingo',
            new vscode.ShellExecution(shellCmd),
            '$dingo'
        );
    }
}

interface DingoTaskDefinition extends vscode.TaskDefinition {
    command: string;      // 'build', 'run', 'go'
    target?: string;      // File or directory to build
    args?: string[];      // Additional arguments
}

/**
 * Register Dingo commands for building and running
 */
export function registerBuildCommands(context: vscode.ExtensionContext) {
    // Build current file command
    context.subscriptions.push(
        vscode.commands.registerCommand('dingo.buildCurrentFile', async () => {
            const editor = vscode.window.activeTextEditor;
            if (!editor || !editor.document.fileName.endsWith('.dingo')) {
                vscode.window.showWarningMessage('Please open a .dingo file to build');
                return;
            }

            await runDingoCommand(context.extensionUri, 'build', editor.document.fileName);
        })
    );

    // Run current file command
    context.subscriptions.push(
        vscode.commands.registerCommand('dingo.runCurrentFile', async () => {
            const editor = vscode.window.activeTextEditor;
            if (!editor || !editor.document.fileName.endsWith('.dingo')) {
                vscode.window.showWarningMessage('Please open a .dingo file to run');
                return;
            }

            await runDingoCommand(context.extensionUri, 'run', editor.document.fileName);
        })
    );

    // Build workspace command
    context.subscriptions.push(
        vscode.commands.registerCommand('dingo.buildWorkspace', async () => {
            const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
            if (!workspaceFolder) {
                vscode.window.showWarningMessage('No workspace folder open');
                return;
            }

            await runDingoCommand(context.extensionUri, 'build', '.', workspaceFolder.uri.fsPath);
        })
    );
}

/**
 * Run a dingo command and show error panel on failure
 */
async function runDingoCommand(
    extensionUri: vscode.Uri,
    command: 'build' | 'run',
    target: string,
    cwd?: string
): Promise<void> {
    const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
    const workingDir = cwd || (workspaceFolder?.uri.fsPath) || path.dirname(target);

    // Show output channel for live output
    const outputChannel = vscode.window.createOutputChannel('Dingo');
    outputChannel.show();
    outputChannel.appendLine(`🐕 Running: dingo ${command} "${target}"`);
    outputChannel.appendLine('');

    return new Promise((resolve) => {
        let output = '';
        let errorOutput = '';

        const process = cp.spawn('dingo', [command, target], {
            cwd: workingDir,
            shell: true
        });

        process.stdout?.on('data', (data: Buffer) => {
            const text = data.toString();
            output += text;
            outputChannel.append(text);
        });

        process.stderr?.on('data', (data: Buffer) => {
            const text = data.toString();
            errorOutput += text;
            outputChannel.append(text);
        });

        process.on('close', (code) => {
            if (code !== 0) {
                const fullOutput = output + errorOutput;
                outputChannel.appendLine('');
                outputChannel.appendLine(`❌ Process exited with code ${code}`);

                // Detect panic or error and show error panel
                if (fullOutput.includes('panic:')) {
                    DingoErrorPanel.show(extensionUri, 'panic', fullOutput, target);
                } else if (command === 'build') {
                    DingoErrorPanel.show(extensionUri, 'build', fullOutput, target);
                } else {
                    DingoErrorPanel.show(extensionUri, 'run', fullOutput, target);
                }
            } else {
                outputChannel.appendLine('');
                outputChannel.appendLine(`✅ ${command === 'build' ? 'Build' : 'Run'} completed successfully`);

                // Auto-hide output channel on success after a short delay
                setTimeout(() => {
                    // Don't hide if user is looking at it
                }, 2000);
            }
            resolve();
        });

        process.on('error', (err) => {
            outputChannel.appendLine(`❌ Failed to start process: ${err.message}`);
            DingoErrorPanel.show(extensionUri, command, `Failed to start dingo: ${err.message}`, target);
            resolve();
        });
    });
}
