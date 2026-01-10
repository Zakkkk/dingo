import * as vscode from 'vscode';
import * as path from 'path';
import * as fs from 'fs';

/**
 * Dingo Debug Configuration Provider
 * Provides debug configurations that build Dingo files and debug with Go's delve
 */
export class DingoDebugConfigurationProvider implements vscode.DebugConfigurationProvider {

    resolveDebugConfiguration(
        folder: vscode.WorkspaceFolder | undefined,
        config: vscode.DebugConfiguration,
        token?: vscode.CancellationToken
    ): vscode.ProviderResult<vscode.DebugConfiguration> {

        // If no config provided, create a default one
        if (!config.type && !config.request && !config.name) {
            const editor = vscode.window.activeTextEditor;
            if (editor && editor.document.fileName.endsWith('.dingo')) {
                config.type = 'dingo';
                config.name = 'Debug Dingo File';
                config.request = 'launch';
                config.program = '${file}';
            }
        }

        if (config.type !== 'dingo') {
            return config;
        }

        // Convert dingo config to go debug config
        return this.resolveDingoConfig(folder, config);
    }

    private async resolveDingoConfig(
        folder: vscode.WorkspaceFolder | undefined,
        config: vscode.DebugConfiguration
    ): Promise<vscode.DebugConfiguration> {

        const workspaceRoot = folder?.uri.fsPath || '';
        const program = config.program || '${file}';

        // Resolve variables in program path
        let resolvedProgram = program;
        if (program === '${file}') {
            const editor = vscode.window.activeTextEditor;
            resolvedProgram = editor?.document.fileName || '';
        } else if (program === '${workspaceFolder}') {
            resolvedProgram = workspaceRoot;
        }

        // Determine output binary path
        // dingo build outputs binary to workspace root with the base name
        let binaryPath: string;

        if (resolvedProgram.endsWith('.dingo')) {
            // Single file - binary goes to workspace root with base name
            // e.g., cmd/api/main.dingo -> workspaceRoot/main
            const baseName = path.basename(resolvedProgram, '.dingo');
            binaryPath = path.join(workspaceRoot, baseName);
        } else {
            // Directory - binary goes to workspace root with directory name
            // e.g., ./cmd/api -> workspaceRoot/api
            const dirName = path.basename(resolvedProgram);
            binaryPath = path.join(workspaceRoot, dirName);
        }

        // Build the Dingo project first using preLaunchTask
        // The task will be defined in tasks.json

        // Return a Go debug configuration that uses delve
        const goConfig: vscode.DebugConfiguration = {
            type: 'go',
            name: config.name || 'Debug Dingo',
            request: 'launch',
            mode: 'exec',
            program: binaryPath,
            args: config.args || [],
            env: config.env || {},
            cwd: config.cwd || workspaceRoot,
            preLaunchTask: config.preLaunchTask || 'dingo: Build Current File',
            // Enable source mapping back to .dingo files
            substitutePath: [
                {
                    from: path.join(workspaceRoot, 'build'),
                    to: workspaceRoot
                }
            ]
        };

        return goConfig;
    }
}

/**
 * Generate default launch.json and tasks.json for a Dingo project
 */
export async function generateDebugConfig(): Promise<void> {
    const workspaceFolder = vscode.workspace.workspaceFolders?.[0];
    if (!workspaceFolder) {
        vscode.window.showErrorMessage('No workspace folder open');
        return;
    }

    const vscodePath = path.join(workspaceFolder.uri.fsPath, '.vscode');

    // Ensure .vscode directory exists
    if (!fs.existsSync(vscodePath)) {
        fs.mkdirSync(vscodePath, { recursive: true });
    }

    // Generate tasks.json
    const tasksPath = path.join(vscodePath, 'tasks.json');
    if (!fs.existsSync(tasksPath)) {
        const tasksConfig = {
            version: '2.0.0',
            tasks: [
                {
                    label: 'dingo: Build Current File',
                    type: 'shell',
                    command: 'dingo',
                    args: ['build', '${file}'],
                    group: {
                        kind: 'build',
                        isDefault: true
                    },
                    problemMatcher: '$dingo',
                    presentation: {
                        reveal: 'always',
                        panel: 'shared'
                    }
                },
                {
                    label: 'dingo: Build Workspace',
                    type: 'shell',
                    command: 'dingo',
                    args: ['build', '.'],
                    group: 'build',
                    problemMatcher: '$dingo'
                },
                {
                    label: 'dingo: Run Current File',
                    type: 'shell',
                    command: 'dingo',
                    args: ['run', '${file}'],
                    group: 'build',
                    problemMatcher: '$dingo',
                    presentation: {
                        reveal: 'always',
                        focus: true
                    }
                }
            ]
        };
        fs.writeFileSync(tasksPath, JSON.stringify(tasksConfig, null, 4));
        vscode.window.showInformationMessage('Created .vscode/tasks.json for Dingo');
    }

    // Generate launch.json
    const launchPath = path.join(vscodePath, 'launch.json');
    if (!fs.existsSync(launchPath)) {
        const launchConfig = {
            version: '0.2.0',
            configurations: [
                {
                    name: 'Debug Dingo File',
                    type: 'go',
                    request: 'launch',
                    mode: 'exec',
                    program: '${workspaceFolder}/build/${fileBasenameNoExtension}',
                    args: [],
                    cwd: '${workspaceFolder}',
                    preLaunchTask: 'dingo: Build Current File'
                },
                {
                    name: 'Debug Dingo Package',
                    type: 'go',
                    request: 'launch',
                    mode: 'exec',
                    program: '${workspaceFolder}/build/${relativeFileDirname}/${fileBasenameNoExtension}',
                    args: [],
                    cwd: '${workspaceFolder}',
                    preLaunchTask: 'dingo: Build Workspace'
                }
            ]
        };
        fs.writeFileSync(launchPath, JSON.stringify(launchConfig, null, 4));
        vscode.window.showInformationMessage('Created .vscode/launch.json for Dingo debugging');
    }
}

export function registerDebugCommands(context: vscode.ExtensionContext) {
    context.subscriptions.push(
        vscode.commands.registerCommand('dingo.generateDebugConfig', generateDebugConfig)
    );
}
