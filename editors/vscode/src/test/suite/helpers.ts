import * as vscode from 'vscode';
import * as path from 'path';

// Get the workspace root (Dingo project root)
export function getWorkspaceRoot(): string {
	const workspaceFolders = vscode.workspace.workspaceFolders;
	if (!workspaceFolders || workspaceFolders.length === 0) {
		throw new Error('No workspace folder open');
	}
	return workspaceFolders[0].uri.fsPath;
}

// Get path to an example file
export function getExamplePath(relativePath: string): string {
	return path.join(getWorkspaceRoot(), 'examples', relativePath);
}

// Wait for the LSP to be ready by checking for diagnostic updates
export async function waitForLSP(timeoutMs: number = 10000): Promise<void> {
	const startTime = Date.now();

	// Poll until we get diagnostics or timeout
	while (Date.now() - startTime < timeoutMs) {
		// Give the LSP time to initialize
		await sleep(500);

		// Check if the dingo extension is active
		const extension = vscode.extensions.getExtension('dingo.dingo');
		if (extension?.isActive) {
			// Extension is active, give it a bit more time to fully initialize
			await sleep(1000);
			return;
		}
	}

	// Even if extension didn't activate, continue (might still work)
	console.warn('LSP may not be fully initialized, continuing anyway');
}

// Sleep for a given number of milliseconds
export function sleep(ms: number): Promise<void> {
	return new Promise(resolve => setTimeout(resolve, ms));
}

// Open a document and wait for it to be visible
export async function openDocument(filePath: string): Promise<vscode.TextDocument> {
	const doc = await vscode.workspace.openTextDocument(filePath);
	await vscode.window.showTextDocument(doc);
	return doc;
}

// Get hover content at a specific position
export async function getHoverAt(
	document: vscode.TextDocument,
	line: number,
	character: number
): Promise<string | null> {
	const position = new vscode.Position(line, character);

	const hovers = await vscode.commands.executeCommand<vscode.Hover[]>(
		'vscode.executeHoverProvider',
		document.uri,
		position
	);

	if (!hovers || hovers.length === 0) {
		return null;
	}

	// Extract text content from hover
	const contents = hovers[0].contents;
	if (contents.length === 0) {
		return null;
	}

	// Handle different content types
	const firstContent = contents[0];
	if (typeof firstContent === 'string') {
		return firstContent;
	} else if ('value' in firstContent) {
		return firstContent.value;
	}

	return null;
}

// Normalize hover text for comparison (remove markdown, trim whitespace)
export function normalizeHover(text: string | null): string {
	if (!text) {
		return '';
	}

	return text
		.replace(/```go\n?/g, '')
		.replace(/```\n?/g, '')
		.trim();
}

// Find the position of text in a document
// Returns the position at the START of the text
export function findTextPosition(
	document: vscode.TextDocument,
	searchText: string,
	occurrence: number = 1
): vscode.Position | null {
	const text = document.getText();
	let count = 0;
	let startIndex = 0;

	while (true) {
		const index = text.indexOf(searchText, startIndex);
		if (index === -1) {
			return null;
		}

		count++;
		if (count === occurrence) {
			return document.positionAt(index);
		}

		startIndex = index + 1;
	}
}

// Find text and get hover at that position
export async function getHoverAtText(
	document: vscode.TextDocument,
	searchText: string,
	occurrence: number = 1
): Promise<{ hover: string | null; position: vscode.Position | null }> {
	const position = findTextPosition(document, searchText, occurrence);
	if (!position) {
		console.log(`[TEST] Could not find "${searchText}" in document`);
		return { hover: null, position: null };
	}

	// Show the actual line content and cursor position for verification
	const lineText = document.lineAt(position.line).text;
	const marker = ' '.repeat(position.character) + '^';
	console.log(`[TEST] Found "${searchText}" at line ${position.line + 1}, col ${position.character + 1}`);
	console.log(`[TEST] Line: ${lineText}`);
	console.log(`[TEST] Pos:  ${marker}`);

	const hover = await getHoverAt(document, position.line, position.character);
	return { hover, position };
}

// Find anchor text, then offset to a specific token within it
// Example: getHoverAtToken(doc, "Result[User, DBError]", "DBError")
// finds "Result[User, DBError]" then positions at "DBError" within it
export async function getHoverAtToken(
	document: vscode.TextDocument,
	anchorText: string,
	targetToken: string,
	anchorOccurrence: number = 1
): Promise<{ hover: string | null; position: vscode.Position | null }> {
	// Find the anchor text first
	const anchorPos = findTextPosition(document, anchorText, anchorOccurrence);
	if (!anchorPos) {
		console.log(`[TEST] Could not find anchor "${anchorText}" in document`);
		return { hover: null, position: null };
	}

	// Find the token within the anchor text
	const tokenOffset = anchorText.indexOf(targetToken);
	if (tokenOffset === -1) {
		console.log(`[TEST] Could not find token "${targetToken}" within anchor "${anchorText}"`);
		return { hover: null, position: null };
	}

	// Calculate the actual position
	const lineText = document.lineAt(anchorPos.line).text;
	const actualChar = anchorPos.character + tokenOffset;
	const position = new vscode.Position(anchorPos.line, actualChar);

	// Show the actual line content and cursor position for verification
	const marker = ' '.repeat(actualChar) + '^';
	console.log(`[TEST] Found "${targetToken}" within "${anchorText}" at line ${position.line + 1}, col ${actualChar + 1}`);
	console.log(`[TEST] Line: ${lineText}`);
	console.log(`[TEST] Pos:  ${marker}`);

	const hover = await getHoverAt(document, position.line, position.character);
	return { hover, position };
}
