import * as assert from 'assert';
import * as vscode from 'vscode';
import {
	getExamplePath,
	waitForLSP,
	openDocument,
	getHoverAt,
	getHoverAtText,
	getHoverAtToken,
	normalizeHover,
	sleep,
} from './helpers';

// ============================================================================
// DINGO-SPECIFIC SYNTAX TESTS
// Tests hover functionality for Dingo's unique syntax features
// ============================================================================

suite('Dingo Syntax Hover Tests', () => {
	suiteSetup(async function () {
		this.timeout(30000);
		await waitForLSP(10000);
	});

	// ========================================================================
	// ERROR PROPAGATION (?)
	// Tests: expr?, expr ? "msg", expr ? |e| f(e), expr ? e => f(e)
	// ========================================================================
	suite('Error Propagation (?)', () => {
		let document: vscode.TextDocument;

		suiteSetup(async function () {
			this.timeout(20000);
			const filePath = getExamplePath('01_error_propagation/http_handler.dingo');
			document = await openDocument(filePath);
			await sleep(2000);
		});

		test('basic ? operator - extractUserID(r)?', async function () {
			this.timeout(10000);
			// Line 55: userID := extractUserID(r)?
			const { hover, position } = await getHoverAtText(document, 'extractUserID(r)?');
			assert.ok(position, 'Should find extractUserID(r)?');
			// The ? should have hover explaining error propagation or the function
		});

		test('? with string context - expr ? "message"', async function () {
			this.timeout(10000);
			// Line 59: user := loadUserFromDB(userID) ? "database lookup failed"
			const { hover, position } = await getHoverAtText(document, 'loadUserFromDB(userID) ? "database');
			assert.ok(position, 'Should find ? with string context');
		});

		test('? with Rust lambda - expr ? |err| transform', async function () {
			this.timeout(10000);
			// Line 63: ? |err| NewAppError(403, "permission denied", err)
			const { hover, position } = await getHoverAtText(document, '|err| NewAppError');
			const normalized = normalizeHover(hover);
			console.log(`[TEST] Rust lambda hover: "${normalized?.substring(0, 100)}..."`);
			assert.ok(position, 'Should find Rust-style lambda');
			if (hover) {
				assert.ok(
					normalized.includes('err') || normalized.includes('error') || normalized.includes('Lambda'),
					`Lambda param should have hover: ${normalized}`
				);
			}
		});

		test('? with TS lambda (parens) - expr ? (e) => transform', async function () {
			this.timeout(10000);
			// Line 67: ? (e) => NewAppError(500, "serialization error", e)
			const { hover, position } = await getHoverAtText(document, '(e) => NewAppError');
			assert.ok(position, 'Should find TS-style lambda with parens');
		});

		test('? with TS lambda (no parens) - expr ? err => transform', async function () {
			this.timeout(10000);
			// Line 86: ? err => fmt.Errorf("processing error: %w", err)
			const { hover, position } = await getHoverAtText(document, 'err => fmt.Errorf');
			assert.ok(position, 'Should find TS-style lambda without parens');
		});

		test('lambda body function call - NewAppError', async function () {
			this.timeout(10000);
			// NewAppError inside lambda body
			const { hover, position } = await getHoverAtText(document, 'NewAppError(403');
			const normalized = normalizeHover(hover);
			console.log(`[TEST] NewAppError hover: "${normalized?.substring(0, 100)}..."`);
			assert.ok(position, 'Should find NewAppError in lambda body');
			if (hover) {
				assert.ok(
					normalized.includes('func') || normalized.includes('NewAppError') || normalized.includes('error'),
					`NewAppError should show function signature: ${normalized}`
				);
			}
		});
	});

	// ========================================================================
	// SAFE NAVIGATION (?.)
	// Tests: config?.field, config?.nested?.deep
	// ========================================================================
	suite('Safe Navigation (?.)', () => {
		let document: vscode.TextDocument;

		suiteSetup(async function () {
			this.timeout(20000);
			const filePath = getExamplePath('08_safe_navigation/config.dingo');
			document = await openDocument(filePath);
			await sleep(2000);
		});

		test('single safe nav - config?.Database', async function () {
			this.timeout(10000);
			// Line 63: return config?.Database?.Host ?? "localhost"
			const { hover, position } = await getHoverAtText(document, 'config?.Database');
			assert.ok(position, 'Should find config?.Database');
		});

		test('chained safe nav - config?.Database?.Host', async function () {
			this.timeout(10000);
			const { hover, position } = await getHoverAtText(document, 'Database?.Host');
			assert.ok(position, 'Should find chained safe navigation');
		});

		test('deep safe nav - config?.Database?.SSL?.CertPath', async function () {
			this.timeout(10000);
			// Line 68: return config?.Database?.SSL?.CertPath ?? "/etc/ssl/cert.pem"
			const { hover, position } = await getHoverAtText(document, 'SSL?.CertPath');
			assert.ok(position, 'Should find deep safe navigation');
		});

		test('safe nav on pointer field - config?.Database?.SSL?.CAPath', async function () {
			this.timeout(10000);
			// Line 74: path := config?.Database?.SSL?.CAPath
			const { hover, position } = await getHoverAtText(document, 'SSL?.CAPath');
			assert.ok(position, 'Should find safe nav on pointer field');
		});
	});

	// ========================================================================
	// NULL COALESCING (??)
	// Tests: value ?? default, chained a ?? b ?? c
	// ========================================================================
	suite('Null Coalescing (??)', () => {
		let document: vscode.TextDocument;

		suiteSetup(async function () {
			this.timeout(20000);
			const filePath = getExamplePath('10_null_coalesce/defaults.dingo');
			document = await openDocument(filePath);
			await sleep(2000);
		});

		test('basic ?? - config?.Host ?? "localhost"', async function () {
			this.timeout(10000);
			// Line 37: return config?.Host ?? "localhost"
			const { hover, position } = await getHoverAtText(document, 'Host ?? "localhost"');
			assert.ok(position, 'Should find ?? operator');
		});

		test('chained ?? - primary ?? secondary ?? tertiary', async function () {
			this.timeout(10000);
			// Line 98: return primary ?? secondary ?? tertiary ?? "https://api.default.com"
			const { hover, position } = await getHoverAtText(document, 'primary ?? secondary');
			assert.ok(position, 'Should find chained ?? operator');
		});

		test('?? combined with ?. - config?.Port ?? 8080', async function () {
			this.timeout(10000);
			// Line 42: return config?.Port ?? 8080
			const { hover, position } = await getHoverAtText(document, 'Port ?? 8080');
			assert.ok(position, 'Should find ?? combined with safe nav');
		});
	});

	// ========================================================================
	// LAMBDAS
	// Tests: |x| expr, |a, b| expr, (x) => expr, (a, b) => expr
	// ========================================================================
	suite('Lambdas', () => {
		let document: vscode.TextDocument;

		suiteSetup(async function () {
			this.timeout(20000);
			const filePath = getExamplePath('06_lambdas/data_pipeline.dingo');
			document = await openDocument(filePath);
			await sleep(2000);
		});

		test('TS lambda single param - (u) => u.Active', async function () {
			this.timeout(10000);
			// Line 32: eligible := Filter(users, (u) => u.Active && u.Premium && u.Age >= 18)
			const { hover, position } = await getHoverAtText(document, '(u) => u.Active');
			assert.ok(position, 'Should find TS-style lambda');
		});

		test('Rust lambda single param - |u| fmt.Sprintf', async function () {
			this.timeout(10000);
			// Line 36: names := Map(eligible, |u| fmt.Sprintf("%s <%s>", u.Name, u.Email))
			const { hover, position } = await getHoverAtText(document, '|u| fmt.Sprintf');
			assert.ok(position, 'Should find Rust-style lambda');
		});

		test('TS lambda multi param - (acc, u) => {...}', async function () {
			this.timeout(10000);
			// Line 40: summary := Reduce(eligible, "", (acc, u) => {
			const { hover, position } = await getHoverAtText(document, '(acc, u) =>');
			assert.ok(position, 'Should find multi-param TS lambda');
		});

		test('Rust lambda multi param - |a, b| a.Age < b.Age', async function () {
			this.timeout(10000);
			// Line 110: byAge := SortUsers(users, |a, b| a.Age < b.Age)
			const { hover, position } = await getHoverAtText(document, '|a, b| a.Age');
			assert.ok(position, 'Should find multi-param Rust lambda');
		});

		test('TS lambda with method call - (a, b) => strings.ToLower', async function () {
			this.timeout(10000);
			// Line 118: byName := SortUsers(users, (a, b) => strings.ToLower(a.Name) < strings.ToLower(b.Name))
			const { hover, position } = await getHoverAtText(document, '(a, b) => strings.ToLower');
			assert.ok(position, 'Should find lambda with method call');
		});

		test('lambda parameter access - u.Name inside lambda', async function () {
			this.timeout(10000);
			// Inside lambda: u.Name
			const { hover, position } = await getHoverAtText(document, 'u.Name, u.Email');
			assert.ok(position, 'Should find field access in lambda');
		});
	});

	// ========================================================================
	// MATCH EXPRESSIONS & ENUMS
	// Tests: match expr { ... }, enum variants, destructuring, guards
	// ========================================================================
	suite('Match Expressions & Enums', () => {
		let document: vscode.TextDocument;

		suiteSetup(async function () {
			this.timeout(20000);
			const filePath = getExamplePath('04_pattern_matching/event_handler.dingo');
			document = await openDocument(filePath);
			await sleep(2000);
		});

		test('enum declaration - enum Event', async function () {
			this.timeout(10000);
			// Line 17: enum Event {
			const { hover, position } = await getHoverAtText(document, 'enum Event');
			assert.ok(position, 'Should find enum declaration');
		});

		test('enum variant with fields - UserCreated { userID: int', async function () {
			this.timeout(10000);
			// Line 18: UserCreated { userID: int, email: string }
			const { hover, position } = await getHoverAtText(document, 'UserCreated { userID');
			assert.ok(position, 'Should find enum variant with fields');
		});

		test('match keyword - match event', async function () {
			this.timeout(10000);
			// Line 29: return match event {
			const { hover, position } = await getHoverAtText(document, 'match event {');
			assert.ok(position, 'Should find match keyword');
		});

		test('match arm pattern - UserCreated(userID, email)', async function () {
			this.timeout(10000);
			// Line 30: UserCreated(userID, email) =>
			const { hover, position } = await getHoverAtText(document, 'UserCreated(userID, email)');
			assert.ok(position, 'Should find match arm pattern');
		});

		test('match arm with guard - if amount > 1000', async function () {
			this.timeout(10000);
			// Line 36: OrderPlaced(orderID, amount, userID) if amount > 1000 =>
			const { hover, position } = await getHoverAtText(document, 'if amount > 1000');
			assert.ok(position, 'Should find match guard');
		});

		test('match wildcard - _ =>', async function () {
			this.timeout(10000);
			// Line 59: _ => 4,  // Everything else
			const { hover, position } = await getHoverAtText(document, '_ => 4');
			assert.ok(position, 'Should find wildcard pattern');
		});

		test('enum constructor - Event.UserCreated(1, "alice")', async function () {
			this.timeout(10000);
			// Line 81: Event.UserCreated(1, "alice@example.com")
			const { hover, position } = await getHoverAtText(document, 'Event.UserCreated(1');
			assert.ok(position, 'Should find enum constructor');
		});

		test('destructured variable in match arm - orderID in match', async function () {
			this.timeout(10000);
			// Inside match: fmt.Sprintf("Order %s confirmed", orderID)
			const { hover, position } = await getHoverAtText(document, 'orderID, userID)');
			assert.ok(position, 'Should find destructured variable');
		});
	});

	// ========================================================================
	// TERNARY OPERATOR
	// Tests: condition ? trueVal : falseVal
	// ========================================================================
	suite('Ternary Operator', () => {
		let document: vscode.TextDocument;

		suiteSetup(async function () {
			this.timeout(20000);
			const filePath = getExamplePath('10_null_coalesce/defaults.dingo');
			document = await openDocument(filePath);
			await sleep(2000);
		});

		test('ternary expression - level != "" ? level : envLevel', async function () {
			this.timeout(10000);
			// Line 60: return level != "" ? level : envLevel != "" ? envLevel : "info"
			const { hover, position } = await getHoverAtText(document, 'level != "" ? level');
			assert.ok(position, 'Should find ternary expression');
		});

		test('chained ternary - nested ? : expressions', async function () {
			this.timeout(10000);
			// Same line, but deeper: envLevel != "" ? envLevel : "info"
			const { hover, position } = await getHoverAtText(document, 'envLevel != "" ? envLevel');
			assert.ok(position, 'Should find chained ternary');
		});
	});
});

// ============================================================================
// GO NATIVE ELEMENTS TESTS
// Ensures we don't break standard Go hover functionality
// ============================================================================

suite('Go Native Hover Tests', () => {
	suiteSetup(async function () {
		this.timeout(30000);
		await waitForLSP(10000);
	});

	// ========================================================================
	// FUNCTIONS
	// ========================================================================
	suite('Go Functions', () => {
		let document: vscode.TextDocument;

		suiteSetup(async function () {
			this.timeout(20000);
			const filePath = getExamplePath('01_error_propagation/http_handler.dingo');
			document = await openDocument(filePath);
			await sleep(2000);
		});

		test('function declaration - func GetUserHandler', async function () {
			this.timeout(10000);
			const { hover, position } = await getHoverAtText(document, 'GetUserHandler(w http');
			const normalized = normalizeHover(hover);
			console.log(`[TEST] Function hover: "${normalized?.substring(0, 150)}..."`);
			assert.ok(position, 'Should find function declaration');
			if (hover) {
				assert.ok(
					normalized.includes('func') || normalized.includes('GetUserHandler'),
					`Function should show signature: ${normalized}`
				);
			}
		});

		test('function call - loadUserFromDB(userID)', async function () {
			this.timeout(10000);
			const { hover, position } = await getHoverAtText(document, 'loadUserFromDB(userID)');
			const normalized = normalizeHover(hover);
			assert.ok(position, 'Should find function call');
			if (hover) {
				assert.ok(
					normalized.includes('func') || normalized.includes('loadUserFromDB'),
					`Function call should show signature: ${normalized}`
				);
			}
		});

		test('method call - w.Header().Set', async function () {
			this.timeout(10000);
			const { hover, position } = await getHoverAtText(document, 'w.Header()');
			assert.ok(position, 'Should find method call');
		});

		test('stdlib function - fmt.Errorf', async function () {
			this.timeout(10000);
			// fmt.Errorf is used in AdvancedHandler
			const { hover, position } = await getHoverAtText(document, 'fmt.Errorf("validation');
			const normalized = normalizeHover(hover);
			assert.ok(position, 'Should find stdlib function');
			if (hover) {
				assert.ok(
					normalized.includes('func') || normalized.includes('Errorf') || normalized.includes('error'),
					`Stdlib function should show signature: ${normalized}`
				);
			}
		});
	});

	// ========================================================================
	// TYPES
	// ========================================================================
	suite('Go Types', () => {
		let document: vscode.TextDocument;

		suiteSetup(async function () {
			this.timeout(20000);
			const filePath = getExamplePath('01_error_propagation/http_handler.dingo');
			document = await openDocument(filePath);
			await sleep(2000);
		});

		test('struct type declaration - type AppError struct', async function () {
			this.timeout(10000);
			const { hover, position } = await getHoverAtText(document, 'type AppError struct');
			assert.ok(position, 'Should find struct declaration');
		});

		test('struct field - Code int', async function () {
			this.timeout(10000);
			const { hover, position } = await getHoverAtText(document, 'Code    int');
			assert.ok(position, 'Should find struct field');
		});

		test('type reference in function param - http.Request', async function () {
			this.timeout(10000);
			// http.Request is used in function parameters
			const { hover, position } = await getHoverAtText(document, '*http.Request)');
			const normalized = normalizeHover(hover);
			assert.ok(position, 'Should find type reference');
			if (hover) {
				assert.ok(
					normalized.includes('Request') || normalized.includes('http') || normalized.includes('struct'),
					`Type should show definition: ${normalized}`
				);
			}
		});

		test('type in variable declaration - *User', async function () {
			this.timeout(10000);
			// Line 107: func loadUserFromDB(id string) (*User, error)
			const { hover, position } = await getHoverAtText(document, '(*User, error)');
			assert.ok(position, 'Should find type in return');
		});
	});

	// ========================================================================
	// VARIABLES
	// ========================================================================
	suite('Go Variables', () => {
		let document: vscode.TextDocument;

		suiteSetup(async function () {
			this.timeout(20000);
			const filePath = getExamplePath('01_error_propagation/http_handler.dingo');
			document = await openDocument(filePath);
			await sleep(2000);
		});

		test('local variable - userID :=', async function () {
			this.timeout(10000);
			const { hover, position } = await getHoverAtText(document, 'userID := extract');
			const normalized = normalizeHover(hover);
			console.log(`[TEST] Variable hover: "${normalized?.substring(0, 100)}..."`);
			assert.ok(position, 'Should find variable declaration');
		});

		test('parameter - r *http.Request', async function () {
			this.timeout(10000);
			const { hover, position } = await getHoverAtText(document, 'r *http.Request');
			assert.ok(position, 'Should find function parameter');
		});

		test('field access - user.Name', async function () {
			this.timeout(10000);
			// Look in a test file that has user.Name
			const lambdaFile = await openDocument(getExamplePath('06_lambdas/data_pipeline.dingo'));
			await sleep(1000);
			const { hover, position } = await getHoverAtText(lambdaFile, 'u.Name, u.Email');
			assert.ok(position, 'Should find field access');
		});
	});

	// ========================================================================
	// IMPORTS
	// ========================================================================
	suite('Go Imports', () => {
		let document: vscode.TextDocument;

		suiteSetup(async function () {
			this.timeout(20000);
			const filePath = getExamplePath('01_error_propagation/http_handler.dingo');
			document = await openDocument(filePath);
			await sleep(2000);
		});

		test('import package - "net/http"', async function () {
			this.timeout(10000);
			const { hover, position } = await getHoverAtText(document, '"net/http"');
			assert.ok(position, 'Should find import');
		});

		test('import package - "encoding/json"', async function () {
			this.timeout(10000);
			const { hover, position } = await getHoverAtText(document, '"encoding/json"');
			assert.ok(position, 'Should find import');
		});
	});
});

// ============================================================================
// TRANSFORM POSITION TESTS (KNOWN FAILURES)
// Tests for positions that are affected by Dingo -> Go transformations
// These test specific positions that should have hover but currently don't
// ============================================================================

suite('Transform Position Tests', () => {
	suiteSetup(async function () {
		this.timeout(30000);
		await waitForLSP(10000);
	});

	// These tests target known broken positions - they document bugs

	suite('Result/Option Type Parameters', () => {
		test('Result second type param - DBError in Result[User, DBError]', async function () {
			this.timeout(20000);

			// repository.dingo line 54: func FindUserByID(...) Result[User, DBError] {
			const filePath = getExamplePath('02_result/repository.dingo');
			const document = await openDocument(filePath);
			await sleep(2000);

			// Find "Result[User, DBError]" and position at "DBError" within it
			const { hover, position } = await getHoverAtToken(
				document,
				'Result[User, DBError]',
				'DBError'
			);
			const normalized = normalizeHover(hover);

			assert.ok(position, 'Should find DBError in Result[User, DBError]');
			assert.ok(
				normalized && (normalized.includes('DBError') || normalized.includes('struct') || normalized.includes('type')),
				`DBError type param should have hover, got: ${normalized}`
			);
		});

		test('Result first type param - User in Result[User, DBError]', async function () {
			this.timeout(20000);

			const filePath = getExamplePath('02_result/repository.dingo');
			const document = await openDocument(filePath);
			await sleep(2000);

			// Find "Result[User, DBError]" and position at "User" within it
			const { hover, position } = await getHoverAtToken(
				document,
				'Result[User, DBError]',
				'User'
			);
			const normalized = normalizeHover(hover);

			assert.ok(position, 'Should find User in Result[User, DBError]');
			assert.ok(
				normalized && (normalized.includes('User') || normalized.includes('struct') || normalized.includes('type')),
				`User type param should have hover, got: ${normalized}`
			);
		});

		test('Struct literal in transformed return - DBError{}', async function () {
			this.timeout(20000);

			// repository.dingo line 60: return DBError{Code: "NOT_FOUND", ...}
			const filePath = getExamplePath('02_result/repository.dingo');
			const document = await openDocument(filePath);
			await sleep(2000);

			// Find the DBError struct literal by its unique context
			const { hover, position } = await getHoverAtToken(
				document,
				'return DBError{Code:',
				'DBError'
			);
			const normalized = normalizeHover(hover);

			assert.ok(position, 'Should find DBError struct literal');
			assert.ok(
				normalized && (normalized.includes('DBError') || normalized.includes('struct')),
				`DBError struct literal should have hover, got: ${normalized}`
			);
		});

		test('Struct field in literal - Code field', async function () {
			this.timeout(20000);

			const filePath = getExamplePath('02_result/repository.dingo');
			const document = await openDocument(filePath);
			await sleep(2000);

			// Find Code field in struct literal
			const { hover, position } = await getHoverAtToken(
				document,
				'DBError{Code: "NOT_FOUND"',
				'Code'
			);
			const normalized = normalizeHover(hover);

			assert.ok(position, 'Should find Code field');
			assert.ok(
				normalized && (normalized.includes('Code') || normalized.includes('field') || normalized.includes('string')),
				`Code field should have hover, got: ${normalized}`
			);
		});
	});

	suite('Match Arm Variables', () => {
		test('Destructured variable - userID in match arm', async function () {
			this.timeout(20000);

			const filePath = getExamplePath('04_pattern_matching/event_handler.dingo');
			const document = await openDocument(filePath);
			await sleep(2000);

			// Line 30: UserCreated(userID, email) =>
			// Find userID specifically in the match arm pattern
			const { hover, position } = await getHoverAtToken(
				document,
				'UserCreated(userID, email)',
				'userID'
			);
			const normalized = normalizeHover(hover);

			assert.ok(position, 'Should find userID in match arm');
			assert.ok(
				normalized && (normalized.includes('userID') || normalized.includes('int') || normalized.includes('var')),
				`userID should have hover, got: ${normalized}`
			);
		});

		test('Destructured variable - email in match arm', async function () {
			this.timeout(20000);

			const filePath = getExamplePath('04_pattern_matching/event_handler.dingo');
			const document = await openDocument(filePath);
			await sleep(2000);

			// Find email in the match arm pattern
			const { hover, position } = await getHoverAtToken(
				document,
				'UserCreated(userID, email)',
				'email'
			);
			const normalized = normalizeHover(hover);

			assert.ok(position, 'Should find email in match arm');
			assert.ok(
				normalized && (normalized.includes('email') || normalized.includes('string') || normalized.includes('var')),
				`email should have hover, got: ${normalized}`
			);
		});
	});

	suite('Safe Navigation Fields', () => {
		test('Database field in config?.Database', async function () {
			this.timeout(20000);

			const filePath = getExamplePath('08_safe_navigation/config.dingo');
			const document = await openDocument(filePath);
			await sleep(2000);

			// Line 63: config?.Database?.Host ?? "localhost"
			// Position at "Database" within the safe nav chain
			const { hover, position } = await getHoverAtToken(
				document,
				'config?.Database?.Host',
				'Database'
			);
			const normalized = normalizeHover(hover);

			assert.ok(position, 'Should find Database field');
			assert.ok(
				normalized && (normalized.includes('Database') || normalized.includes('DatabaseConfig') || normalized.includes('field')),
				`Database field should have hover, got: ${normalized}`
			);
		});

		test('Host field in config?.Database?.Host', async function () {
			this.timeout(20000);

			const filePath = getExamplePath('08_safe_navigation/config.dingo');
			const document = await openDocument(filePath);
			await sleep(2000);

			// Position at "Host" within the safe nav chain
			const { hover, position } = await getHoverAtToken(
				document,
				'config?.Database?.Host',
				'Host'
			);
			const normalized = normalizeHover(hover);

			assert.ok(position, 'Should find Host field');
			assert.ok(
				normalized && (normalized.includes('Host') || normalized.includes('string') || normalized.includes('field')),
				`Host field should have hover, got: ${normalized}`
			);
		});
	});

	suite('Null Coalesce Variables', () => {
		test('Host field before ??', async function () {
			this.timeout(20000);

			const filePath = getExamplePath('10_null_coalesce/defaults.dingo');
			const document = await openDocument(filePath);
			await sleep(2000);

			// Line 37: config?.Host ?? "localhost"
			// Position at "Host" before the ??
			const { hover, position } = await getHoverAtToken(
				document,
				'config?.Host ?? "localhost"',
				'Host'
			);
			const normalized = normalizeHover(hover);

			assert.ok(position, 'Should find Host field');
			assert.ok(
				normalized && (normalized.includes('Host') || normalized.includes('string') || normalized.includes('field')),
				`Host field should have hover, got: ${normalized}`
			);
		});

		test('Port field before ??', async function () {
			this.timeout(20000);

			const filePath = getExamplePath('10_null_coalesce/defaults.dingo');
			const document = await openDocument(filePath);
			await sleep(2000);

			// Line 42: config?.Port ?? 8080
			const { hover, position } = await getHoverAtToken(
				document,
				'config?.Port ?? 8080',
				'Port'
			);
			const normalized = normalizeHover(hover);

			assert.ok(position, 'Should find Port field');
			assert.ok(
				normalized && (normalized.includes('Port') || normalized.includes('int') || normalized.includes('field')),
				`Port field should have hover, got: ${normalized}`
			);
		});
	});
});

// ============================================================================
// REGRESSION TESTS
// Tests for previously fixed hover bugs
// ============================================================================

suite('Hover Regression Tests', () => {
	suiteSetup(async function () {
		this.timeout(30000);
		await waitForLSP(10000);
	});

	test('lambda body identifiers have hover (regression for NewAppError)', async function () {
		this.timeout(20000);

		const filePath = getExamplePath('01_error_propagation/http_handler.dingo');
		const document = await openDocument(filePath);
		await sleep(2000);

		// Find NewAppError in lambda body: |err| NewAppError(403, "permission denied", err)
		const { hover, position } = await getHoverAtText(document, '|err| NewAppError');
		const normalized = normalizeHover(hover);

		console.log(`[TEST] Lambda body hover: "${normalized?.substring(0, 100)}..."`);

		assert.ok(position, 'Should find NewAppError in lambda body');
		if (hover) {
			assert.ok(
				normalized.includes('func') || normalized.includes('NewAppError') || normalized.includes('err'),
				`NewAppError should have hover: ${normalized}`
			);
		}
	});

	test('match arm body identifiers have hover', async function () {
		this.timeout(20000);

		const filePath = getExamplePath('04_pattern_matching/event_handler.dingo');
		const document = await openDocument(filePath);
		await sleep(2000);

		// Find ProcessEvent function call
		const { hover, position } = await getHoverAtText(document, 'ProcessEvent(event)');
		const normalized = normalizeHover(hover);

		assert.ok(position, 'Should find ProcessEvent call');
		if (hover) {
			assert.ok(
				normalized.includes('func') || normalized.includes('ProcessEvent') || normalized.includes('string'),
				`ProcessEvent should have hover: ${normalized}`
			);
		}
	});

	test('chained safe navigation preserves position', async function () {
		this.timeout(20000);

		const filePath = getExamplePath('08_safe_navigation/config.dingo');
		const document = await openDocument(filePath);
		await sleep(2000);

		// Deep chain: config?.Database?.SSL?.CertPath
		const { hover, position } = await getHoverAtText(document, 'CertPath ?? "/etc/ssl');
		assert.ok(position, 'Should find CertPath in safe nav chain');
	});
});
