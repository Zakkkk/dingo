import * as vscode from 'vscode';
import { GoldenFileSupport } from './goldenFileSupport';
import { activateLSPClient, deactivateLSPClient } from './lspClient';
import { DingoTaskProvider, registerBuildCommands } from './taskProvider';
import { registerDebugCommands } from './debugProvider';
import { registerDingoDebugger } from './dingoDebugAdapter';

export async function activate(context: vscode.ExtensionContext) {
    console.log('Dingo extension activating...');

    // Ensure .dingo files always use 'dingo' language mode (prevents Go extension interference)
    context.subscriptions.push(
        vscode.workspace.onDidOpenTextDocument(doc => {
            if (doc.fileName.endsWith('.dingo') && doc.languageId !== 'dingo') {
                vscode.languages.setTextDocumentLanguage(doc, 'dingo');
                console.log(`Dingo: Set language mode to 'dingo' for ${doc.fileName}`);
            }
        })
    );

    // Also check currently open .dingo files
    for (const doc of vscode.workspace.textDocuments) {
        if (doc.fileName.endsWith('.dingo') && doc.languageId !== 'dingo') {
            vscode.languages.setTextDocumentLanguage(doc, 'dingo');
            console.log(`Dingo: Set language mode to 'dingo' for ${doc.fileName}`);
        }
    }

    // Disable Go extension features for .dingo files to prevent duplicate hovers/definitions
    await configureGoExtensionForDingo();

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

/**
 * Configure Go extension to not interfere with .dingo files
 * This sets workspace configuration to disable Go LSP for .dingo language
 */
async function configureGoExtensionForDingo(): Promise<void> {
    // The Go extension checks the 'go.useLanguageServer' setting per language
    // We set it to false for [dingo] language scope at workspace level
    // This is more reliable than configurationDefaults which only sets defaults

    if (!vscode.workspace.workspaceFolders?.length) {
        return;
    }

    try {
        const config = vscode.workspace.getConfiguration();
        const dingoSettings = config.inspect('[dingo]');

        // Check if we already have workspace-level [dingo] settings with go.useLanguageServer
        const workspaceValue = dingoSettings?.workspaceValue as Record<string, unknown> | undefined;
        if (workspaceValue?.['go.useLanguageServer'] === false) {
            // Already configured
            return;
        }

        // Merge with existing [dingo] settings
        const existingSettings = (workspaceValue || {}) as Record<string, unknown>;
        const newSettings = {
            ...existingSettings,
            'go.useLanguageServer': false
        };

        await config.update('[dingo]', newSettings, vscode.ConfigurationTarget.Workspace);
        console.log('Dingo: Configured workspace to disable Go LSP for .dingo files');
    } catch (error) {
        // Silently fail - this is a best-effort optimization
        console.log('Dingo: Could not update workspace settings:', error);
    }
}
