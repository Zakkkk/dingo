import * as vscode from 'vscode';
import { GoldenFileSupport } from './goldenFileSupport';
import { activateLSPClient, deactivateLSPClient } from './lspClient';
import { DingoTaskProvider, registerBuildCommands } from './taskProvider';
import { registerDebugCommands } from './debugProvider';
import { registerDingoDebugger } from './dingoDebugAdapter';

export async function activate(context: vscode.ExtensionContext) {
    console.log('Dingo extension activating...');

    // Activate LSP client
    await activateLSPClient(context);

    // Register task provider for build/run tasks
    const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
    const taskProvider = new DingoTaskProvider(workspaceRoot);
    context.subscriptions.push(
        vscode.tasks.registerTaskProvider(DingoTaskProvider.DingoType, taskProvider)
    );

    // Register Dingo debugger (delegates to Go debugger)
    registerDingoDebugger(context);

    // Register build and debug commands
    registerBuildCommands(context);
    registerDebugCommands(context);

    // Command: Compare with source/golden file
    const goldenFileSupport = new GoldenFileSupport();
    context.subscriptions.push(
        vscode.commands.registerCommand('dingo.compareWithSource', () => {
            goldenFileSupport.compareWithSource();
        })
    );

    console.log('Dingo extension activated');
}

export async function deactivate() {
    // Deactivate LSP client
    await deactivateLSPClient();
}
