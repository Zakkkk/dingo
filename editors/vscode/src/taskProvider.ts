import * as vscode from 'vscode';
import * as path from 'path';

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

            const terminal = vscode.window.createTerminal('Dingo Build');
            terminal.show();
            terminal.sendText(`dingo build "${editor.document.fileName}"`);
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

            const terminal = vscode.window.createTerminal('Dingo Run');
            terminal.show();
            terminal.sendText(`dingo run "${editor.document.fileName}"`);
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

            const terminal = vscode.window.createTerminal('Dingo Build');
            terminal.show();
            terminal.sendText(`cd "${workspaceFolder.uri.fsPath}" && dingo build .`);
        })
    );
}
