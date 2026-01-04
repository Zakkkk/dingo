import * as assert from 'assert';
import * as vscode from 'vscode';
import * as path from 'path';
import {
	getWorkspaceRoot,
	waitForLSP,
	sleep,
} from './helpers';

// ============================================================================
// DIAGNOSTIC TESTS
// Tests for diagnostic behavior including the edit cycle bug
// ============================================================================

suite('Diagnostic Tests', () => {
	suiteSetup(async function () {
		this.timeout(30000);
		await waitForLSP(10000);
	});

	// ========================================================================
	// EDIT CYCLE TEST
	// Tests: valid -> invalid -> valid (diagnostics should clear)
	// This tests for the "stale diagnostic" / FileSet accumulation bug
	// ========================================================================
	suite('Edit Cycle (FileSet Accumulation Bug)', () => {
		const validContent = `package main

// Test: Lambda syntax error - missing type annotation
// Expected: Error should clear when fixed

func test() {
    fn := |x: int| x + 1
    _ = fn
}

func main() {}
`;

		const invalidContent = `package main

// Test: Lambda syntax error - missing type annotation
// Expected: Error should clear when fixed

func test() {
    fn := |x| x + 1
    _ = fn
}

func main() {}
`;

		let testFilePath: string;
		let originalContent: string;

		suiteSetup(async function () {
			this.timeout(10000);
			testFilePath = path.join(getWorkspaceRoot(), 'tests/lsp/04_lambda_error/main.dingo');

			// Save original content
			const doc = await vscode.workspace.openTextDocument(testFilePath);
			originalContent = doc.getText();
		});

		suiteTeardown(async function () {
			this.timeout(10000);
			// Restore original content
			const doc = await vscode.workspace.openTextDocument(testFilePath);
			const edit = new vscode.WorkspaceEdit();
			edit.replace(
				doc.uri,
				new vscode.Range(0, 0, doc.lineCount, 0),
				originalContent
			);
			await vscode.workspace.applyEdit(edit);
			await doc.save();
		});

		test('edit cycle: valid -> invalid -> valid (no stale diagnostics)', async function () {
			this.timeout(60000);

			// Step 1: Open file with valid content
			console.log('[TEST] Step 1: Opening file with VALID content');
			const doc = await vscode.workspace.openTextDocument(testFilePath);
			await vscode.window.showTextDocument(doc);

			// Set valid content
			const edit1 = new vscode.WorkspaceEdit();
			edit1.replace(
				doc.uri,
				new vscode.Range(0, 0, doc.lineCount, 0),
				validContent
			);
			await vscode.workspace.applyEdit(edit1);
			await sleep(3000); // Wait for diagnostics

			let diags = vscode.languages.getDiagnostics(doc.uri);
			let parseErrors = diags.filter(d =>
				d.severity === vscode.DiagnosticSeverity.Error &&
				!d.source?.includes('unusedfunc')
			);
			console.log(`[TEST] Step 1 diagnostics: ${diags.length} total, ${parseErrors.length} parse errors`);

			// Step 1 should have no parse errors (warnings like "unused function" are OK)
			assert.strictEqual(parseErrors.length, 0,
				`Step 1: Expected no parse errors, got: ${parseErrors.map(d => d.message).join(', ')}`);

			// Step 2: Change to invalid content
			console.log('[TEST] Step 2: Changing to INVALID content');
			const edit2 = new vscode.WorkspaceEdit();
			edit2.replace(
				doc.uri,
				new vscode.Range(0, 0, doc.lineCount, 0),
				invalidContent
			);
			await vscode.workspace.applyEdit(edit2);
			await sleep(3000);

			diags = vscode.languages.getDiagnostics(doc.uri);
			console.log(`[TEST] Step 2 diagnostics: ${diags.length} total`);
			diags.forEach(d => console.log(`[TEST]   - Line ${d.range.start.line + 1}: ${d.message}`));

			// Step 3: Change back to valid content
			console.log('[TEST] Step 3: Changing back to VALID content');
			const edit3 = new vscode.WorkspaceEdit();
			edit3.replace(
				doc.uri,
				new vscode.Range(0, 0, doc.lineCount, 0),
				validContent
			);
			await vscode.workspace.applyEdit(edit3);
			await sleep(3000);

			diags = vscode.languages.getDiagnostics(doc.uri);
			parseErrors = diags.filter(d =>
				d.severity === vscode.DiagnosticSeverity.Error &&
				!d.source?.includes('unusedfunc')
			);
			console.log(`[TEST] Step 3 diagnostics: ${diags.length} total, ${parseErrors.length} parse errors`);
			diags.forEach(d => console.log(`[TEST]   - Line ${d.range.start.line + 1}: [${d.severity}] ${d.message}`));

			// Check specifically for the FileSet accumulation bug
			const fileSetBug = diags.find(d =>
				d.message.includes('expected declaration') &&
				d.message.includes('package')
			);
			assert.ok(!fileSetBug,
				`FileSet accumulation bug detected! Error on line ${fileSetBug?.range.start.line}: ${fileSetBug?.message}`);

			// Step 3 should have no parse errors
			assert.strictEqual(parseErrors.length, 0,
				`Step 3: Expected no parse errors after fixing, got: ${parseErrors.map(d => `Line ${d.range.start.line + 1}: ${d.message}`).join(', ')}`);
		});

		test('rapid edit cycle (stress test for FileSet)', async function () {
			this.timeout(60000);

			console.log('[TEST] Rapid edit cycle stress test');
			const doc = await vscode.workspace.openTextDocument(testFilePath);
			await vscode.window.showTextDocument(doc);

			// Do 5 rapid cycles
			for (let i = 0; i < 5; i++) {
				// Invalid
				const editInvalid = new vscode.WorkspaceEdit();
				editInvalid.replace(
					doc.uri,
					new vscode.Range(0, 0, doc.lineCount, 0),
					invalidContent
				);
				await vscode.workspace.applyEdit(editInvalid);
				await sleep(500);

				// Valid
				const editValid = new vscode.WorkspaceEdit();
				editValid.replace(
					doc.uri,
					new vscode.Range(0, 0, doc.lineCount, 0),
					validContent
				);
				await vscode.workspace.applyEdit(editValid);
				await sleep(500);
			}

			// Final wait for diagnostics to settle
			await sleep(3000);

			const diags = vscode.languages.getDiagnostics(doc.uri);
			console.log(`[TEST] After ${5} cycles: ${diags.length} diagnostics`);
			diags.forEach(d => console.log(`[TEST]   - Line ${d.range.start.line + 1}: ${d.message}`));

			// Check for FileSet bug
			const fileSetBug = diags.find(d =>
				d.message.includes('expected declaration') &&
				d.message.includes('package')
			);
			assert.ok(!fileSetBug,
				`FileSet accumulation bug after rapid cycling! Line ${fileSetBug?.range.start.line}: ${fileSetBug?.message}`);

			// Check that line numbers are reasonable (not accumulated)
			const badLineNumber = diags.find(d => d.range.start.line > 20);
			assert.ok(!badLineNumber,
				`Suspiciously high line number (${badLineNumber?.range.start.line}) suggests FileSet pollution`);
		});
	});
});
