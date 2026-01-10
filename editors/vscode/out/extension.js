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
//# sourceMappingURL=extension.js.map