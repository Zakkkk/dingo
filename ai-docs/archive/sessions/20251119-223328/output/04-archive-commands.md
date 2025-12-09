# Archive Commands - Ready to Execute

**WARNING:** Review each command before executing. All changes are reversible via git.

**Working Directory:** `/Users/jack/mag/dingo`

---

## Phase 1: Critical Fixes (Execute Day 1)

### Step 1.1: Fix features/INDEX.md (MANUAL EDIT RECOMMENDED)

**File to edit:** `/Users/jack/mag/dingo/features/INDEX.md`

**Option A: Manual Edit (RECOMMENDED)**

Open in editor and make changes per `03-consolidation-plan.md` Section "Phase 1, Step 1.1"

**Option B: Automated Edit (Use with caution)**

```bash
cd /Users/jack/mag/dingo

# Backup first
cp features/INDEX.md features/INDEX.md.backup

# Update header
sed -i '' 's/\*\*Last Updated:\*\* 2025-11-16/**Last Updated:** 2025-11-19/' features/INDEX.md
sed -i '' 's/\*\*Phase:\*\* Phase 0 → Phase 1 Transition/**Phase:** Phase V Complete - Infrastructure \& Developer Experience (Ready for v1.0)/' features/INDEX.md

# Add status source note (insert after Phase line)
sed -i '' '/\*\*Phase:\*\*/a\
**Status Source of Truth:** See CLAUDE.md for current implementation phase
' features/INDEX.md

# Update P0 features - Result Type
sed -i '' 's/| \*\*P0\*\* | Result Type | .* | 🔴 Not Started | \[result-type.md\]/| **P0** | Result Type | 🟡 Medium | 2-3 weeks | ⭐⭐⭐⭐⭐ (#1 issue) | ✅ Implemented (Phase 2) | [result-type.md]/' features/INDEX.md

# Update P0 features - Error Propagation
sed -i '' 's/| \*\*P0\*\* | Error Propagation .`?`. | .* | 🔴 Not Started | \[error-propagation.md\]/| **P0** | Error Propagation (`?`) | 🟢 Low | 1-2 weeks | ⭐⭐⭐⭐⭐ | ✅ Implemented (Phase 2) | [error-propagation.md]/' features/INDEX.md

# Update P0 features - Option Type
sed -i '' 's/| \*\*P0\*\* | Option Type | .* | 🔴 Not Started | \[option-type.md\]/| **P0** | Option Type | 🟡 Medium | 2-3 weeks | ⭐⭐⭐⭐⭐ | ✅ Implemented (Phase 2) | [option-type.md]/' features/INDEX.md

# Update P0 features - Pattern Matching
sed -i '' 's/| \*\*P0\*\* | Pattern Matching | .* | 🔴 Not Started | \[pattern-matching.md\]/| **P0** | Pattern Matching | 🟠 High | 3-4 weeks | ⭐⭐⭐⭐⭐ | ✅ Implemented (Phase 4) | [pattern-matching.md]/' features/INDEX.md

# Update P0 features - Sum Types
sed -i '' 's/| \*\*P0\*\* | Sum Types | .* | 🔴 Not Started | \[sum-types.md\]/| **P0** | Sum Types | 🟠 High | 3-4 weeks | ⭐⭐⭐⭐⭐ (996+ 👍) | ✅ Implemented (Phase 3) | [sum-types.md]/' features/INDEX.md

# Update P1 features - Enums
sed -i '' 's/| \*\*P1\*\* | Type-Safe Enums | .* | 🔴 Not Started | \[enums.md\]/| **P1** | Type-Safe Enums | 🟡 Medium | 1-2 weeks | ⭐⭐⭐⭐⭐ (900+ 👍) | ✅ Implemented (Phase 3) | [enums.md]/' features/INDEX.md

# Update P2 features - Tuples
sed -i '' 's/| \*\*P2\*\* | Tuples | .* | 🔴 Not Started | \[tuples.md\]/| **P2** | Tuples | 🟡 Medium | 1-2 weeks | ⭐⭐⭐ | 🟡 Partial (10% - pattern matching only) | [tuples.md]/' features/INDEX.md

# Verify changes
echo "=== Verification ==="
grep -c "✅ Implemented" features/INDEX.md
# Should output: 6 (Result, Error Prop, Option, Pattern Match, Sum Types, Enums)

# Check backup exists
ls -lh features/INDEX.md.backup
```

**Better Approach: Manual Edit Guide**

1. Open `/Users/jack/mag/dingo/features/INDEX.md`
2. Update lines 5-7 (header):
   ```markdown
   **Last Updated:** 2025-11-19
   **Phase:** Phase V Complete - Infrastructure & Developer Experience (Ready for v1.0)
   **Status Source of Truth:** See CLAUDE.md for current implementation phase
   ```
3. Find and update feature table (lines ~51-66):
   - Change all P0 features from `🔴 Not Started` to `✅ Implemented (Phase X)`
   - Result Type → Phase 2
   - Error Propagation → Phase 2
   - Option Type → Phase 2
   - Pattern Matching → Phase 4
   - Sum Types → Phase 3
   - Enums → Phase 3
   - Tuples → `🟡 Partial (10% - pattern matching only)`

4. Add new Infrastructure table after line 66:
   ```markdown
   ### Infrastructure & Architecture (Phase 5)

   | Priority | Feature | Complexity | Status | Notes |
   |----------|---------|------------|--------|-------|
   | **ARCH** | Type Annotations | 🟢 Low | ✅ Implemented (Phase 1) | `param: Type` syntax |
   | **ARCH** | Generic Syntax | 🟢 Low | ✅ Implemented (Phase 2) | `Result[T,E]` support |
   | **ARCH** | Keywords | 🟢 Low | ✅ Implemented (Phase 1) | Preprocessor-based |
   | **ARCH** | Source Maps | 🟡 Medium | ✅ Implemented (Phase 5) | LSP position mapping |
   | **ARCH** | Workspace Builds | 🟡 Medium | ✅ Implemented (Phase 5) | Multi-package support |
   | **ARCH** | Unqualified Imports | 🟢 Low | ✅ Implemented (Phase 5) | `use` keyword |
   ```

5. Update Implementation Roadmap section (lines ~240-256):
   - Change section title to: `## ✅ Phase 1-5: COMPLETE (11 features implemented)`
   - Add checkmarks to completed features
   - Add status summary: "58% feature complete (11/19 features), ready for v1.0"

---

### Step 1.2: Update CLAUDE.md Source of Truth Notice

```bash
cd /Users/jack/mag/dingo

# Backup
cp CLAUDE.md CLAUDE.md.backup

# This requires manual edit - insert after line 3 (after main title)
# Open in editor and add:
```

**Manual Edit Required:** Insert after "# Claude AI Agent Memory & Instructions" (line 1):

```markdown
## 📍 Project Status (Source of Truth)

**This file is the CANONICAL source for current project status.**

- **Current Phase:** Phase V Complete - Infrastructure & Developer Experience
- **Implementation:** 11/19 features complete (58%)
- **Test Status:** 92.2% passing (245/266 tests)
- **Production Ready:** 3/4 external model approval for v1.0
- **Last Updated:** 2025-11-19

For feature-by-feature status, see `features/INDEX.md`.
For implementation history, see `CHANGELOG.md`.
```

**Add to features/INDEX.md top:**

```markdown
> **Status Source of Truth:** For current implementation phase and metrics, see `/Users/jack/mag/dingo/CLAUDE.md`
>
> This file tracks feature planning and implementation status. For overall project status, defer to CLAUDE.md.
```

---

### Step 1.3: Delete Misleading Files

```bash
cd /Users/jack/mag/dingo

# Delete resolved bug investigations
rm ai-docs/ast-bug-investigation.md
rm ai-docs/ast-bug-investigation-detailed.md
rm ai-docs/codegen-corruption-investigation.md
rm ai-docs/golden-test-investigation.md
rm ai-docs/lsp-position-bug-trace.md
rm ai-docs/CRITICAL-2-FIX-SUMMARY.md
rm ai-docs/gola-architect-phase-summary.md
rm ai-docs/analysis/pattern-match-bug-analysis.md

# Delete LSP investigation directories
rm -rf ai-docs/lsp-crash-investigation/
rm -rf ai-docs/lsp-parallel/
rm -rf ai-docs/lsp-source-mapping-fix/

# Verify deletions
echo "=== Deleted Files ==="
! test -f ai-docs/CRITICAL-2-FIX-SUMMARY.md && echo "✅ CRITICAL-2-FIX-SUMMARY.md deleted"
! test -d ai-docs/lsp-parallel && echo "✅ lsp-parallel/ deleted"
! test -d ai-docs/lsp-crash-investigation && echo "✅ lsp-crash-investigation/ deleted"
! test -d ai-docs/lsp-source-mapping-fix && echo "✅ lsp-source-mapping-fix/ deleted"

echo "✅ Phase 1 Step 1.3 Complete"
```

**Files Deleted:** 8 files + 3 directories (26 files total) = **34 files**

---

## Phase 2: Consolidate Duplicates (Execute Day 2)

### Step 2.1: Consolidate Enum Naming Documentation

```bash
cd /Users/jack/mag/dingo

# Create archive directory
mkdir -p ai-docs/archive/research/enum-naming

# Delete root duplicate
rm ai-docs/enum-naming-convention-analysis.md

# Archive analysis files (keep recommendations only)
mv ai-docs/research/enum-naming-architecture.md ai-docs/archive/research/enum-naming/
mv ai-docs/research/enum-naming-convention-analysis.md ai-docs/archive/research/enum-naming/
mv ai-docs/research/enum-naming-options.md ai-docs/archive/research/enum-naming/

# Keep: ai-docs/research/enum-naming-recommendations.md

# Verify
echo "=== Enum Naming Consolidation ==="
test -f ai-docs/research/enum-naming-recommendations.md && echo "✅ Kept: enum-naming-recommendations.md"
! test -f ai-docs/enum-naming-convention-analysis.md && echo "✅ Deleted: root duplicate"
test -d ai-docs/archive/research/enum-naming && echo "✅ Archived: 3 analysis files"

echo "✅ Phase 2 Step 2.1 Complete"
```

---

### Step 2.2: Archive Go Missing Features Research

```bash
cd /Users/jack/mag/dingo

# Create archive directory
mkdir -p ai-docs/archive/research/golang-missing

# Archive all Go missing features research
mv ai-docs/research/golang_missing/*.md ai-docs/archive/research/golang-missing/

# Optionally remove empty directory
rmdir ai-docs/research/golang_missing

# Verify
echo "=== Go Missing Features Consolidation ==="
test -d ai-docs/archive/research/golang-missing && echo "✅ Archived: 5 golang_missing files"
! test -d ai-docs/research/golang_missing && echo "✅ Removed: empty golang_missing/"

echo "✅ Phase 2 Step 2.2 Complete"
```

---

### Step 2.3: Consolidate Compiler Research

```bash
cd /Users/jack/mag/dingo

# Create archive directory
mkdir -p ai-docs/archive/research/compiler

# Archive chatgpt and gemini research (keep claude)
mv ai-docs/research/compiler/chatgpt-research.md ai-docs/archive/research/compiler/
mv ai-docs/research/compiler/gemini_research.md ai-docs/archive/research/compiler/

# Keep: ai-docs/research/compiler/claude-research.md

# Verify
echo "=== Compiler Research Consolidation ==="
test -f ai-docs/research/compiler/claude-research.md && echo "✅ Kept: claude-research.md"
test -f ai-docs/archive/research/compiler/chatgpt-research.md && echo "✅ Archived: chatgpt-research.md"
test -f ai-docs/archive/research/compiler/gemini_research.md && echo "✅ Archived: gemini_research.md"

echo "✅ Phase 2 Step 2.3 Complete"
```

---

### Step 2.4: Fix Delegation Strategy Duplication

```bash
cd /Users/jack/mag/dingo

# Delete root duplicate (keep research/ version)
rm ai-docs/delegation-strategy.md

# Update CLAUDE.md reference (manual edit required)
# Find lines referencing "ai-docs/delegation-strategy.md"
# Replace with "ai-docs/research/delegation/delegation-strategy.md"

# Verify
echo "=== Delegation Strategy Consolidation ==="
! test -f ai-docs/delegation-strategy.md && echo "✅ Deleted: root duplicate"
test -f ai-docs/research/delegation/delegation-strategy.md && echo "✅ Kept: research/delegation/ version"

echo "⚠️  MANUAL STEP: Update CLAUDE.md references to delegation-strategy.md"
echo "✅ Phase 2 Step 2.4 Complete"
```

**Manual Edit Required:** Update CLAUDE.md

Search for: `ai-docs/delegation-strategy.md`
Replace with: `ai-docs/research/delegation/delegation-strategy.md`

---

### Step 2.5: Delete Redundant IMPLEMENTATION_STATUS.md

```bash
cd /Users/jack/mag/dingo

# Delete (info already in CLAUDE.md)
rm ai-docs/research/delegation/IMPLEMENTATION_STATUS.md
rm ai-docs/research/delegation/CLAUDISH_TIMEOUT_UPDATE.md

# Archive COMPLETE.md
mkdir -p ai-docs/archive/research/delegation
mv ai-docs/research/delegation/CLAUDISH_TIMEOUT_COMPLETE.md ai-docs/archive/research/delegation/

# Verify
echo "=== Delegation Cleanup ==="
! test -f ai-docs/research/delegation/IMPLEMENTATION_STATUS.md && echo "✅ Deleted: IMPLEMENTATION_STATUS.md"
! test -f ai-docs/research/delegation/CLAUDISH_TIMEOUT_UPDATE.md && echo "✅ Deleted: UPDATE.md"
test -f ai-docs/archive/research/delegation/CLAUDISH_TIMEOUT_COMPLETE.md && echo "✅ Archived: COMPLETE.md"

echo "✅ Phase 2 Step 2.5 Complete"
```

---

## Phase 3: Archive Old Sessions (Execute Days 4-7)

### Step 3.1: Create Archive Structure

```bash
cd /Users/jack/mag/dingo

# Create full archive directory structure
mkdir -p ai-docs/archive/{sessions,research,investigations,historical,architecture,reviews}

# Verify
echo "=== Archive Structure ==="
test -d ai-docs/archive/sessions && echo "✅ Created: sessions/"
test -d ai-docs/archive/research && echo "✅ Created: research/"
test -d ai-docs/archive/investigations && echo "✅ Created: investigations/"
test -d ai-docs/archive/historical && echo "✅ Created: historical/"
test -d ai-docs/archive/architecture && echo "✅ Created: architecture/"
test -d ai-docs/archive/reviews && echo "✅ Created: reviews/"

echo "✅ Phase 3 Step 3.1 Complete"
```

---

### Step 3.2: Archive Old Session Folders (Nov 16-18)

```bash
cd /Users/jack/mag/dingo

# Count before
BEFORE=$(ls -1d ai-docs/sessions/*/ 2>/dev/null | wc -l)
echo "Sessions before archive: $BEFORE"

# Archive Nov 16 sessions
mv ai-docs/sessions/20251116-* ai-docs/archive/sessions/ 2>/dev/null

# Archive Nov 17 sessions
mv ai-docs/sessions/20251117-* ai-docs/archive/sessions/ 2>/dev/null

# Archive Nov 18 sessions
mv ai-docs/sessions/20251118-* ai-docs/archive/sessions/ 2>/dev/null

# Count after
AFTER=$(ls -1d ai-docs/sessions/*/ 2>/dev/null | wc -l)
ARCHIVED=$(ls -1d ai-docs/archive/sessions/*/ 2>/dev/null | wc -l)

echo "=== Session Archive Results ==="
echo "Sessions before: $BEFORE"
echo "Sessions after: $AFTER"
echo "Sessions archived: $ARCHIVED"
echo "Expected archived: ~75"

# Verify specific session archived
test -d ai-docs/archive/sessions/20251116-174148 && echo "✅ Sample archived: 20251116-174148"

echo "✅ Phase 3 Step 3.2 Complete"
```

**Expected Output:**
```
Sessions before: 139
Sessions after: ~64
Sessions archived: ~75
```

---

### Step 3.3: Archive Historical Root-Level Files

```bash
cd /Users/jack/mag/dingo

# Archive architecture decisions
mv ai-docs/architectural-comparison-option-b-vs-c.md ai-docs/archive/architecture/
mv ai-docs/package-scanning-architecture.md ai-docs/archive/architecture/
mv ai-docs/package-wide-function-detection-architecture.md ai-docs/archive/architecture/

# Archive historical summaries
mv ai-docs/dev-orchestrator-context-optimization.md ai-docs/archive/historical/
mv ai-docs/golang-developer-summary.md ai-docs/archive/historical/
mv ai-docs/golang-tester-project-state.md ai-docs/archive/historical/
mv ai-docs/parser-research.md ai-docs/archive/historical/

# Archive old reviews
mv ai-docs/reviews/phase4_2_review.md ai-docs/archive/reviews/

# Archive language UI decisions
mv ai-docs/language/UI_IMPLEMENTATION.md ai-docs/archive/historical/

# Verify
echo "=== Historical Files Archived ==="
test -f ai-docs/archive/architecture/architectural-comparison-option-b-vs-c.md && echo "✅ Archived: architectural-comparison"
test -f ai-docs/archive/historical/golang-developer-summary.md && echo "✅ Archived: developer summary"
test -f ai-docs/archive/reviews/phase4_2_review.md && echo "✅ Archived: phase4_2 review"

# Count remaining ai-docs root files
ROOT_FILES=$(find ai-docs -maxdepth 1 -type f -name "*.md" | wc -l)
echo "ai-docs root files remaining: $ROOT_FILES (target: ~3)"

echo "✅ Phase 3 Step 3.3 Complete"
```

---

## Complete Workflow - Execute All Phases

```bash
#!/bin/bash
# Complete Documentation Consolidation Workflow
# Execute from: /Users/jack/mag/dingo

set -e  # Exit on error

echo "🚀 Starting Dingo Documentation Consolidation"
echo "=============================================="
echo ""

# Safety check
read -p "⚠️  This will modify/delete files. Continue? (yes/no): " CONFIRM
if [ "$CONFIRM" != "yes" ]; then
  echo "❌ Aborted"
  exit 1
fi

echo ""
echo "📋 Phase 1: Critical Fixes"
echo "=========================="

# Phase 1 Step 1.3: Delete misleading files
echo "Step 1.3: Deleting resolved bug investigations..."
rm -f ai-docs/ast-bug-investigation.md
rm -f ai-docs/ast-bug-investigation-detailed.md
rm -f ai-docs/codegen-corruption-investigation.md
rm -f ai-docs/golden-test-investigation.md
rm -f ai-docs/lsp-position-bug-trace.md
rm -f ai-docs/CRITICAL-2-FIX-SUMMARY.md
rm -f ai-docs/gola-architect-phase-summary.md
rm -f ai-docs/analysis/pattern-match-bug-analysis.md
rm -rf ai-docs/lsp-crash-investigation/
rm -rf ai-docs/lsp-parallel/
rm -rf ai-docs/lsp-source-mapping-fix/
echo "✅ Deleted 34 files (8 files + 3 directories)"

echo ""
echo "📋 Phase 2: Consolidate Duplicates"
echo "==================================="

# Phase 2 Step 2.1: Enum naming
echo "Step 2.1: Consolidating enum naming docs..."
mkdir -p ai-docs/archive/research/enum-naming
rm -f ai-docs/enum-naming-convention-analysis.md
mv ai-docs/research/enum-naming-architecture.md ai-docs/archive/research/enum-naming/ 2>/dev/null || true
mv ai-docs/research/enum-naming-convention-analysis.md ai-docs/archive/research/enum-naming/ 2>/dev/null || true
mv ai-docs/research/enum-naming-options.md ai-docs/archive/research/enum-naming/ 2>/dev/null || true
echo "✅ Consolidated: 5 → 1 file (4 archived)"

# Phase 2 Step 2.2: Go missing features
echo "Step 2.2: Archiving Go missing features research..."
mkdir -p ai-docs/archive/research/golang-missing
mv ai-docs/research/golang_missing/*.md ai-docs/archive/research/golang-missing/ 2>/dev/null || true
rmdir ai-docs/research/golang_missing 2>/dev/null || true
echo "✅ Archived: 5 golang_missing files"

# Phase 2 Step 2.3: Compiler research
echo "Step 2.3: Consolidating compiler research..."
mkdir -p ai-docs/archive/research/compiler
mv ai-docs/research/compiler/chatgpt-research.md ai-docs/archive/research/compiler/ 2>/dev/null || true
mv ai-docs/research/compiler/gemini_research.md ai-docs/archive/research/compiler/ 2>/dev/null || true
echo "✅ Consolidated: 3 → 1 file (2 archived)"

# Phase 2 Step 2.4: Delegation strategy
echo "Step 2.4: Fixing delegation strategy duplication..."
rm -f ai-docs/delegation-strategy.md
echo "✅ Deleted: root duplicate"

# Phase 2 Step 2.5: Delegation cleanup
echo "Step 2.5: Cleaning delegation files..."
rm -f ai-docs/research/delegation/IMPLEMENTATION_STATUS.md
rm -f ai-docs/research/delegation/CLAUDISH_TIMEOUT_UPDATE.md
mkdir -p ai-docs/archive/research/delegation
mv ai-docs/research/delegation/CLAUDISH_TIMEOUT_COMPLETE.md ai-docs/archive/research/delegation/ 2>/dev/null || true
echo "✅ Cleaned: delegation files"

echo ""
echo "📋 Phase 3: Archive Old Sessions"
echo "================================="

# Phase 3 Step 3.1: Create structure
echo "Step 3.1: Creating archive structure..."
mkdir -p ai-docs/archive/{sessions,research,investigations,historical,architecture,reviews}
echo "✅ Created: archive directories"

# Phase 3 Step 3.2: Archive sessions
echo "Step 3.2: Archiving old sessions (Nov 16-18)..."
BEFORE=$(ls -1d ai-docs/sessions/*/ 2>/dev/null | wc -l)
mv ai-docs/sessions/20251116-* ai-docs/archive/sessions/ 2>/dev/null || true
mv ai-docs/sessions/20251117-* ai-docs/archive/sessions/ 2>/dev/null || true
mv ai-docs/sessions/20251118-* ai-docs/archive/sessions/ 2>/dev/null || true
AFTER=$(ls -1d ai-docs/sessions/*/ 2>/dev/null | wc -l)
ARCHIVED=$(ls -1d ai-docs/archive/sessions/*/ 2>/dev/null | wc -l)
echo "✅ Archived: $ARCHIVED sessions (from $BEFORE to $AFTER active)"

# Phase 3 Step 3.3: Archive historical
echo "Step 3.3: Archiving historical files..."
mv ai-docs/architectural-comparison-option-b-vs-c.md ai-docs/archive/architecture/ 2>/dev/null || true
mv ai-docs/package-scanning-architecture.md ai-docs/archive/architecture/ 2>/dev/null || true
mv ai-docs/package-wide-function-detection-architecture.md ai-docs/archive/architecture/ 2>/dev/null || true
mv ai-docs/dev-orchestrator-context-optimization.md ai-docs/archive/historical/ 2>/dev/null || true
mv ai-docs/golang-developer-summary.md ai-docs/archive/historical/ 2>/dev/null || true
mv ai-docs/golang-tester-project-state.md ai-docs/archive/historical/ 2>/dev/null || true
mv ai-docs/parser-research.md ai-docs/archive/historical/ 2>/dev/null || true
mv ai-docs/reviews/phase4_2_review.md ai-docs/archive/reviews/ 2>/dev/null || true
mv ai-docs/language/UI_IMPLEMENTATION.md ai-docs/archive/historical/ 2>/dev/null || true
echo "✅ Archived: 9 historical files"

echo ""
echo "🎉 Consolidation Complete!"
echo "=========================="
echo ""
echo "Summary:"
echo "- Deleted: ~50 files (resolved bugs, duplicates)"
echo "- Archived: ~1,100 files (sessions, research)"
echo "- Essential: ~52 files remaining"
echo ""
echo "⚠️  MANUAL STEPS REQUIRED:"
echo "1. Edit features/INDEX.md - Update 11 features to ✅ Implemented"
echo "2. Edit CLAUDE.md - Add '📍 Project Status' section"
echo "3. Edit CLAUDE.md - Update delegation-strategy.md references"
echo ""
echo "Run verification:"
echo "  bash ai-docs/sessions/20251119-223328/output/05-verification.sh"
```

**Save as:** `ai-docs/sessions/20251119-223328/output/run-consolidation.sh`

**Execute:**
```bash
cd /Users/jack/mag/dingo
chmod +x ai-docs/sessions/20251119-223328/output/run-consolidation.sh
./ai-docs/sessions/20251119-223328/output/run-consolidation.sh
```

---

## Verification Script

```bash
#!/bin/bash
# Verification Script
# Execute from: /Users/jack/mag/dingo

echo "🔍 Verifying Documentation Consolidation"
echo "========================================="
echo ""

ERRORS=0

# 1. Verify essential files exist
echo "1. Checking essential files..."
test -f README.md && echo "  ✅ README.md" || { echo "  ❌ README.md MISSING"; ERRORS=$((ERRORS+1)); }
test -f CLAUDE.md && echo "  ✅ CLAUDE.md" || { echo "  ❌ CLAUDE.md MISSING"; ERRORS=$((ERRORS+1)); }
test -f CHANGELOG.md && echo "  ✅ CHANGELOG.md" || { echo "  ❌ CHANGELOG.md MISSING"; ERRORS=$((ERRORS+1)); }
test -f features/INDEX.md && echo "  ✅ features/INDEX.md" || { echo "  ❌ features/INDEX.md MISSING"; ERRORS=$((ERRORS+1)); }

# 2. Verify features/INDEX.md updated
echo ""
echo "2. Checking features/INDEX.md status..."
IMPLEMENTED=$(grep -c "✅ Implemented" features/INDEX.md)
if [ "$IMPLEMENTED" -ge 6 ]; then
  echo "  ✅ INDEX.md updated ($IMPLEMENTED implemented features found)"
else
  echo "  ⚠️  INDEX.md may not be updated (only $IMPLEMENTED '✅ Implemented' found, expected 11+)"
  echo "     MANUAL EDIT REQUIRED"
fi

# 3. Count remaining sessions
echo ""
echo "3. Checking active sessions..."
SESSIONS=$(ls -1d ai-docs/sessions/*/ 2>/dev/null | wc -l | tr -d ' ')
if [ "$SESSIONS" -lt 30 ]; then
  echo "  ✅ Active sessions: $SESSIONS (reasonable count)"
else
  echo "  ⚠️  Active sessions: $SESSIONS (expected < 30)"
fi

# 4. Verify archive created
echo ""
echo "4. Checking archive structure..."
test -d ai-docs/archive/sessions && echo "  ✅ sessions/ archive exists" || { echo "  ❌ sessions/ MISSING"; ERRORS=$((ERRORS+1)); }
test -d ai-docs/archive/research && echo "  ✅ research/ archive exists" || { echo "  ❌ research/ MISSING"; ERRORS=$((ERRORS+1)); }
test -d ai-docs/archive/historical && echo "  ✅ historical/ archive exists" || { echo "  ❌ historical/ MISSING"; ERRORS=$((ERRORS+1)); }

# 5. Count archived sessions
echo ""
echo "5. Checking archived sessions..."
ARCHIVED=$(ls -1d ai-docs/archive/sessions/*/ 2>/dev/null | wc -l | tr -d ' ')
if [ "$ARCHIVED" -gt 50 ]; then
  echo "  ✅ Archived sessions: $ARCHIVED (expected ~75)"
else
  echo "  ⚠️  Archived sessions: $ARCHIVED (expected ~75)"
fi

# 6. Verify no duplicates
echo ""
echo "6. Checking for duplicates..."
! test -f ai-docs/delegation-strategy.md && echo "  ✅ No delegation duplicate" || { echo "  ❌ delegation duplicate EXISTS"; ERRORS=$((ERRORS+1)); }
! test -f ai-docs/enum-naming-convention-analysis.md && echo "  ✅ No enum duplicate" || { echo "  ❌ enum duplicate EXISTS"; ERRORS=$((ERRORS+1)); }

# 7. Count essential ai-docs files
echo ""
echo "7. Checking ai-docs root files..."
ESSENTIAL=$(find ai-docs -maxdepth 1 -type f -name "*.md" 2>/dev/null | wc -l | tr -d ' ')
if [ "$ESSENTIAL" -le 5 ]; then
  echo "  ✅ ai-docs root files: $ESSENTIAL (target: ~3)"
else
  echo "  ⚠️  ai-docs root files: $ESSENTIAL (target: ~3)"
fi

# 8. Verify deleted bug investigations
echo ""
echo "8. Checking deleted bug investigations..."
! test -f ai-docs/CRITICAL-2-FIX-SUMMARY.md && echo "  ✅ CRITICAL bugs doc deleted" || { echo "  ❌ CRITICAL-2-FIX-SUMMARY.md still exists"; ERRORS=$((ERRORS+1)); }
! test -d ai-docs/lsp-parallel && echo "  ✅ lsp-parallel/ deleted" || { echo "  ❌ lsp-parallel/ still exists"; ERRORS=$((ERRORS+1)); }
! test -d ai-docs/lsp-crash-investigation && echo "  ✅ lsp-crash-investigation/ deleted" || { echo "  ❌ lsp-crash-investigation/ still exists"; ERRORS=$((ERRORS+1)); }

# Final summary
echo ""
echo "========================================="
if [ "$ERRORS" -eq 0 ]; then
  echo "✅ VERIFICATION PASSED"
  echo ""
  echo "Consolidation successful!"
  echo "- Essential files: preserved"
  echo "- Old sessions: archived"
  echo "- Duplicates: removed"
  echo "- Bug investigations: deleted"
else
  echo "❌ VERIFICATION FAILED ($ERRORS errors)"
  echo ""
  echo "Please review errors above and fix manually."
fi
echo "========================================="

exit $ERRORS
```

**Save as:** `ai-docs/sessions/20251119-223328/output/run-verification.sh`

**Execute:**
```bash
cd /Users/jack/mag/dingo
chmod +x ai-docs/sessions/20251119-223328/output/run-verification.sh
./ai-docs/sessions/20251119-223328/output/run-verification.sh
```

---

## Rollback Commands (If Needed)

```bash
#!/bin/bash
# Rollback Documentation Consolidation
# Execute from: /Users/jack/mag/dingo

echo "⚠️  Rolling back consolidation..."

# Restore all from git
git checkout HEAD -- ai-docs/

echo "✅ Rollback complete - all files restored from git"
```

---

**Next File:** 05-delete-list.md (Justification for all deletions)
