"use strict";
var __createBinding = (this && this.__createBinding) || (Object.create ? (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    var desc = Object.getOwnPropertyDescriptor(m, k);
    if (!desc || ("get" in desc ? !m.__esModule : desc.writable || desc.configurable)) {
      desc = { enumerable: true, get: function() { return m[k]; } };
    }
    Object.defineProperty(o, k2, desc);
}) : (function(o, m, k, k2) {
    if (k2 === undefined) k2 = k;
    o[k2] = m[k];
}));
var __setModuleDefault = (this && this.__setModuleDefault) || (Object.create ? (function(o, v) {
    Object.defineProperty(o, "default", { enumerable: true, value: v });
}) : function(o, v) {
    o["default"] = v;
});
var __importStar = (this && this.__importStar) || (function () {
    var ownKeys = function(o) {
        ownKeys = Object.getOwnPropertyNames || function (o) {
            var ar = [];
            for (var k in o) if (Object.prototype.hasOwnProperty.call(o, k)) ar[ar.length] = k;
            return ar;
        };
        return ownKeys(o);
    };
    return function (mod) {
        if (mod && mod.__esModule) return mod;
        var result = {};
        if (mod != null) for (var k = ownKeys(mod), i = 0; i < k.length; i++) if (k[i] !== "default") __createBinding(result, mod, k[i]);
        __setModuleDefault(result, mod);
        return result;
    };
})();
Object.defineProperty(exports, "__esModule", { value: true });
exports.activate = activate;
exports.deactivate = deactivate;
const vscode = __importStar(require("vscode"));
const goldenFileSupport_1 = require("./goldenFileSupport");
const lspClient_1 = require("./lspClient");
const taskProvider_1 = require("./taskProvider");
const debugProvider_1 = require("./debugProvider");
const dingoDebugAdapter_1 = require("./dingoDebugAdapter");
async function activate(context) {
    console.log('Dingo extension activating...');
    // Register formatter FIRST (before LSP) to ensure VS Code recognizes this extension as a formatter
    // The formatter delegates to LSP when available, or returns empty edits if LSP isn't ready
    const formatterDisposable = vscode.languages.registerDocumentFormattingEditProvider({ scheme: 'file', language: 'dingo' }, {
        async provideDocumentFormattingEdits(document) {
            const client = (0, lspClient_1.getLSPClient)();
            if (!client) {
                console.log('Dingo formatter: LSP client not available');
                vscode.window.showWarningMessage('Dingo LSP is not running. Cannot format.');
                return [];
            }
            try {
                const result = await client.sendRequest('textDocument/formatting', {
                    textDocument: { uri: document.uri.toString() },
                    options: {
                        tabSize: 4,
                        insertSpaces: false
                    }
                });
                if (!result || !Array.isArray(result)) {
                    return [];
                }
                return result.map((edit) => {
                    return new vscode.TextEdit(new vscode.Range(edit.range.start.line, edit.range.start.character, edit.range.end.line, edit.range.end.character), edit.newText);
                });
            }
            catch (error) {
                console.error('Dingo formatting error:', error);
                vscode.window.showErrorMessage(`Dingo formatting failed: ${error}`);
                return [];
            }
        }
    });
    context.subscriptions.push(formatterDisposable);
    console.log('Dingo formatter registered');
    // Ensure .dingo files always use 'dingo' language mode (prevents Go extension interference)
    context.subscriptions.push(vscode.workspace.onDidOpenTextDocument(doc => {
        if (doc.fileName.endsWith('.dingo') && doc.languageId !== 'dingo') {
            vscode.languages.setTextDocumentLanguage(doc, 'dingo');
            console.log(`Dingo: Set language mode to 'dingo' for ${doc.fileName}`);
        }
    }));
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
    await (0, lspClient_1.activateLSPClient)(context);
    // Register task provider for build/run tasks
    const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
    const taskProvider = new taskProvider_1.DingoTaskProvider(workspaceRoot);
    context.subscriptions.push(vscode.tasks.registerTaskProvider(taskProvider_1.DingoTaskProvider.DingoType, taskProvider));
    // Register Dingo debugger (delegates to Go debugger)
    (0, dingoDebugAdapter_1.registerDingoDebugger)(context);
    // Register build and debug commands
    (0, taskProvider_1.registerBuildCommands)(context);
    (0, debugProvider_1.registerDebugCommands)(context);
    // Command: Compare with source/golden file
    const goldenFileSupport = new goldenFileSupport_1.GoldenFileSupport();
    context.subscriptions.push(vscode.commands.registerCommand('dingo.compareWithSource', () => {
        goldenFileSupport.compareWithSource();
    }));
    console.log('Dingo extension activated');
}
async function deactivate() {
    // Deactivate LSP client
    await (0, lspClient_1.deactivateLSPClient)();
}
/**
 * Configure Go extension to not interfere with .dingo files
 * This sets workspace configuration to disable Go LSP for .dingo language
 */
async function configureGoExtensionForDingo() {
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
        const workspaceValue = dingoSettings?.workspaceValue;
        if (workspaceValue?.['go.useLanguageServer'] === false) {
            // Already configured
            return;
        }
        // Merge with existing [dingo] settings
        const existingSettings = (workspaceValue || {});
        const newSettings = {
            ...existingSettings,
            'go.useLanguageServer': false
        };
        await config.update('[dingo]', newSettings, vscode.ConfigurationTarget.Workspace);
        console.log('Dingo: Configured workspace to disable Go LSP for .dingo files');
    }
    catch (error) {
        // Silently fail - this is a best-effort optimization
        console.log('Dingo: Could not update workspace settings:', error);
    }
}
//# sourceMappingURL=extension.js.map