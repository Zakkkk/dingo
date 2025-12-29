import * as path from 'path';
import Mocha from 'mocha';
import { glob } from 'glob';

export async function run(): Promise<void> {
	// Create the mocha test runner
	const mocha = new Mocha({
		ui: 'tdd',
		color: true,
		timeout: 60000, // 60 seconds per test (LSP needs time to initialize)
	});

	const testsRoot = path.resolve(__dirname, '.');

	// Find all test files
	const files = await glob('**/*.test.js', { cwd: testsRoot });

	// Add files to the test suite
	for (const f of files) {
		mocha.addFile(path.resolve(testsRoot, f));
	}

	return new Promise<void>((resolve, reject) => {
		try {
			// Run the mocha tests
			mocha.run((failures: number) => {
				if (failures > 0) {
					reject(new Error(`${failures} tests failed.`));
				} else {
					resolve();
				}
			});
		} catch (err) {
			console.error(err);
			reject(err);
		}
	});
}
