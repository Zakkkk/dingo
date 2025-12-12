import * as vscode from 'vscode';
import { GoldenFileSupport } from './goldenFileSupport';
import { activateLSPClient, deactivateLSPClient } from './lspClient';

export async function activate(context: vscode.ExtensionContext) {
    console.log('Dingo extension activating...');

    // Activate LSP client
    await activateLSPClient(context);

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
