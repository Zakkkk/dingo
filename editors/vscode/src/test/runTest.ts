import * as path from 'path';
import { runTests } from '@vscode/test-electron';

async function main() {
	try {
		// The folder containing the Extension Manifest package.json
		// __dirname at runtime is out/test/, so ../../ gets us to editors/vscode/
		const extensionDevelopmentPath = path.resolve(__dirname, '../../');

		// The path to the test runner script
		const extensionTestsPath = path.resolve(__dirname, './suite/index');

		// The workspace to open for tests (use the Dingo project root)
		// From editors/vscode/, go up two levels to get to dingo project root
		const testWorkspace = path.resolve(extensionDevelopmentPath, '../../');

		console.log('Extension path:', extensionDevelopmentPath);
		console.log('Tests path:', extensionTestsPath);
		console.log('Workspace:', testWorkspace);

		// Download VS Code, unzip it and run the integration tests
		await runTests({
			extensionDevelopmentPath,
			extensionTestsPath,
			launchArgs: [
				testWorkspace,
				'--disable-gpu',           // Reduce resource usage
				'--disable-workspace-trust', // Don't prompt for workspace trust
				'--skip-welcome',           // Skip welcome page
				'--skip-release-notes',     // Skip release notes
			],
		});
	} catch (err) {
		console.error('Failed to run tests:', err);
		process.exit(1);
	}
}

main();
