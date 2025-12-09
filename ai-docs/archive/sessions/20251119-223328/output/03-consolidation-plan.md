# Documentation Consolidation Plan

**Goal:** Reduce from 1,167 files to ~52 essential docs (95.5% reduction)

**Timeline:** 3 phases over 2 weeks

**Status Tracking:** This plan

---

## Target Post-Consolidation Structure

```
/Users/jack/mag/dingo/
├── README.md                      # User-facing intro
├── CLAUDE.md                      # AI instructions (SOURCE OF TRUTH for status)
├── CHANGELOG.md                   # Historical record
│
├── .claude/ (12 files)            # Agent configs (DON'T TOUCH)
│   ├── agents/ (4 files)
│   ├── commands/ (1 file)
│   └── skills/ (7 files)
│
├── features/ (19 files)           # Feature specifications
│   ├── INDEX.md                   # ✅ FIXED: Accurate status
│   └── [18 feature specs]
│
├── ai-docs/                       # AI context (MINIMAL)
│   ├── README.md                  # Index
│   ├── ARCHITECTURE.md            # Technical overview
│   ├── language/
│   │   └── SYNTAX_DESIGN.md       # Syntax reference
│   ├── prompts/
│   │   └── agent-anti-recursion-rules.md
│   ├── research/
│   │   ├── delegation/
│   │   │   └── delegation-strategy.md
│   │   └── enum-naming-recommendations.md  # Final decision only
│   └── archive/                   # Historical docs
│       ├── sessions/              # Old session logs
│       ├── research/              # Old research
│       ├── investigations/        # Resolved bugs
│       └── historical/            # Misc old docs
│
├── tests/golden/
│   └── README.md                  # Test guidelines
│
├── pkg/ (3 READMEs)               # Package documentation
├── examples/ (4 READMEs)          # Example documentation
└── editors/ (3 files)             # Editor integration docs
```

**Total Essential Files:** 52 (down from 1,167)

---

## Phase 1: Critical Fixes (Day 1 - Immediate)

**Priority:** 🔴 CRITICAL - Fix user-facing confusion

### Step 1.1: Fix features/INDEX.md Status

**File:** `/Users/jack/mag/dingo/features/INDEX.md`

**Changes Required:**

1. **Update Header:**
```diff
- **Last Updated:** 2025-11-16
- **Phase:** Phase 0 → Phase 1 Transition
+ **Last Updated:** 2025-11-19
+ **Phase:** Phase V Complete - Infrastructure & Developer Experience (Ready for v1.0)
+ **Status Source of Truth:** See CLAUDE.md for current implementation phase
```

2. **Update Feature Status Table:**

**P0 Features (ALL COMPLETE):**
```diff
- | **P0** | Result Type | 🟡 Medium | 2-3 weeks | ⭐⭐⭐⭐⭐ (#1 issue) | 🔴 Not Started | [result-type.md](./result-type.md) |
+ | **P0** | Result Type | 🟡 Medium | 2-3 weeks | ⭐⭐⭐⭐⭐ (#1 issue) | ✅ Implemented (Phase 2) | [result-type.md](./result-type.md) |

- | **P0** | Error Propagation (`?`) | 🟢 Low | 1-2 weeks | ⭐⭐⭐⭐⭐ | 🔴 Not Started | [error-propagation.md](./error-propagation.md) |
+ | **P0** | Error Propagation (`?`) | 🟢 Low | 1-2 weeks | ⭐⭐⭐⭐⭐ | ✅ Implemented (Phase 2) | [error-propagation.md](./error-propagation.md) |

- | **P0** | Option Type | 🟡 Medium | 2-3 weeks | ⭐⭐⭐⭐⭐ | 🔴 Not Started | [option-type.md](./option-type.md) |
+ | **P0** | Option Type | 🟡 Medium | 2-3 weeks | ⭐⭐⭐⭐⭐ | ✅ Implemented (Phase 2) | [option-type.md](./option-type.md) |

- | **P0** | Pattern Matching | 🟠 High | 3-4 weeks | ⭐⭐⭐⭐⭐ | 🔴 Not Started | [pattern-matching.md](./pattern-matching.md) |
+ | **P0** | Pattern Matching | 🟠 High | 3-4 weeks | ⭐⭐⭐⭐⭐ | ✅ Implemented (Phase 4) | [pattern-matching.md](./pattern-matching.md) |

- | **P0** | Sum Types | 🟠 High | 3-4 weeks | ⭐⭐⭐⭐⭐ (996+ 👍) | 🔴 Not Started | [sum-types.md](./sum-types.md) |
+ | **P0** | Sum Types | 🟠 High | 3-4 weeks | ⭐⭐⭐⭐⭐ (996+ 👍) | ✅ Implemented (Phase 3) | [sum-types.md](./sum-types.md) |
```

**P1 Features (Enums Complete):**
```diff
- | **P1** | Type-Safe Enums | 🟡 Medium | 1-2 weeks | ⭐⭐⭐⭐⭐ (900+ 👍) | 🔴 Not Started | [enums.md](./enums.md) |
+ | **P1** | Type-Safe Enums | 🟡 Medium | 1-2 weeks | ⭐⭐⭐⭐⭐ (900+ 👍) | ✅ Implemented (Phase 3) | [enums.md](./enums.md) |
```

**Architecture Features (6 Complete):**

Add new table section:
```markdown
### Infrastructure & Architecture (Phase 5)

| Priority | Feature | Complexity | Timeline | Status | Notes |
|----------|---------|------------|----------|--------|-------|
| **ARCH** | Type Annotations | 🟢 Low | 1 week | ✅ Implemented (Phase 1) | `param: Type` syntax |
| **ARCH** | Generic Syntax | 🟢 Low | 1 week | ✅ Implemented (Phase 2) | `Result[T,E]` support |
| **ARCH** | Keywords | 🟢 Low | 1 week | ✅ Implemented (Phase 1) | Preprocessor-based |
| **ARCH** | Source Maps | 🟡 Medium | 2 weeks | ✅ Implemented (Phase 5) | LSP position mapping |
| **ARCH** | Workspace Builds | 🟡 Medium | 2 weeks | ✅ Implemented (Phase 5) | Multi-package support |
| **ARCH** | Unqualified Imports | 🟢 Low | 1 week | ✅ Implemented (Phase 5) | `use` keyword |
```

**P2 Features (Tuples Partial):**
```diff
- | **P2** | Tuples | 🟡 Medium | 1-2 weeks | ⭐⭐⭐ | 🔴 Not Started | [tuples.md](./tuples.md) |
+ | **P2** | Tuples | 🟡 Medium | 1-2 weeks | ⭐⭐⭐ | 🟡 Partial (10% - pattern matching only) | [tuples.md](./tuples.md) |
```

3. **Update Implementation Roadmap Section:**
```diff
## Implementation Roadmap (Updated)

- ### Phase 1: Core Error Handling (MVP) - 8-10 weeks
+ ### ✅ Phase 1-5: COMPLETE (11 features implemented)

**Critical Path:**
- 1. Sum Types (3-4 weeks) - Foundation for Result/Option
- 2. Result Type (2-3 weeks) - Depends on sum types
- 3. Option Type (2-3 weeks) - Depends on sum types
- 4. Pattern Matching (3-4 weeks) - Needed for ergonomic Result/Option usage
- 5. Error Propagation (1-2 weeks) - Sugar on top of Result
+ 1. ✅ Sum Types (Phase 3) - Foundation for Result/Option
+ 2. ✅ Result Type (Phase 2) - Depends on sum types
+ 3. ✅ Option Type (Phase 2) - Depends on sum types
+ 4. ✅ Pattern Matching (Phase 4) - Needed for ergonomic Result/Option usage
+ 5. ✅ Error Propagation (Phase 2) - Sugar on top of Result
+ 6. ✅ Enums (Phase 3) - Type-safe enums
+ 7. ✅ Infrastructure (Phase 5) - Source maps, workspace builds, LSP
+
+ **Status:** 58% feature complete (11/19 features), ready for v1.0
```

**Manual Edit Required:** See `output/04-archive-commands.md` for sed script

**Verification:**
```bash
# After edit, verify:
grep "✅ Implemented" /Users/jack/mag/dingo/features/INDEX.md | wc -l
# Should output: 11
```

**Time Estimate:** 30 minutes (manual edit recommended for accuracy)

---

### Step 1.2: Establish Single Source of Truth

**Update CLAUDE.md:**

Add prominent notice at top:
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

**Update features/INDEX.md:**

Add at top:
```markdown
> **Status Source of Truth:** For current implementation phase and metrics, see `/Users/jack/mag/dingo/CLAUDE.md`
>
> This file tracks feature planning and implementation status. For overall project status, defer to CLAUDE.md.
```

**Time Estimate:** 10 minutes

---

### Step 1.3: Delete Misleading Files

**Delete Resolved Bug Investigations:**

```bash
rm /Users/jack/mag/dingo/ai-docs/ast-bug-investigation.md
rm /Users/jack/mag/dingo/ai-docs/ast-bug-investigation-detailed.md
rm /Users/jack/mag/dingo/ai-docs/codegen-corruption-investigation.md
rm /Users/jack/mag/dingo/ai-docs/golden-test-investigation.md
rm /Users/jack/mag/dingo/ai-docs/lsp-position-bug-trace.md
rm /Users/jack/mag/dingo/ai-docs/CRITICAL-2-FIX-SUMMARY.md
rm /Users/jack/mag/dingo/ai-docs/gola-architect-phase-summary.md
rm /Users/jack/mag/dingo/ai-docs/analysis/pattern-match-bug-analysis.md
```

**Delete LSP Investigation Directories:**
```bash
rm -rf /Users/jack/mag/dingo/ai-docs/lsp-crash-investigation/
rm -rf /Users/jack/mag/dingo/ai-docs/lsp-parallel/
rm -rf /Users/jack/mag/dingo/ai-docs/lsp-source-mapping-fix/
```

**Files Deleted:** 8 files + 3 directories (26 files) = **34 files**

**Time Estimate:** 5 minutes

---

**Phase 1 Total Time:** ~45 minutes

**Phase 1 Result:**
- ✅ features/INDEX.md shows accurate 58% completion
- ✅ CLAUDE.md established as status source of truth
- ✅ 34 misleading files removed
- ✅ Users no longer confused about project state

---

## Phase 2: Consolidate Duplicates (Days 2-3)

**Priority:** 🟠 HIGH - Reduce maintenance burden

### Step 2.1: Consolidate Enum Naming Documentation

**Current State:** 5 files on enum naming

**Action:**

1. **KEEP:** `ai-docs/research/enum-naming-recommendations.md` (final decision)
2. **DELETE:** Root duplicate
```bash
rm /Users/jack/mag/dingo/ai-docs/enum-naming-convention-analysis.md
```
3. **ARCHIVE:** Analysis files
```bash
mkdir -p /Users/jack/mag/dingo/ai-docs/archive/research/enum-naming/
mv /Users/jack/mag/dingo/ai-docs/research/enum-naming-architecture.md ai-docs/archive/research/enum-naming/
mv /Users/jack/mag/dingo/ai-docs/research/enum-naming-convention-analysis.md ai-docs/archive/research/enum-naming/
mv /Users/jack/mag/dingo/ai-docs/research/enum-naming-options.md ai-docs/archive/research/enum-naming/
```

**Files Reduced:** 5 → 1 (keep) + 3 (archive)

**Time Estimate:** 10 minutes

---

### Step 2.2: Archive Go Missing Features Research

**Current State:** 5 files analyzing Go proposals

**Action:**

All findings consolidated into `features/INDEX.md`, archive originals:

```bash
mkdir -p /Users/jack/mag/dingo/ai-docs/archive/research/golang-missing/
mv /Users/jack/mag/dingo/ai-docs/research/golang_missing/*.md ai-docs/archive/research/golang-missing/
```

**Files Archived:** 5 files

**Rationale:** Research complete, findings in INDEX.md, keep for historical reference

**Time Estimate:** 5 minutes

---

### Step 2.3: Consolidate Compiler Research

**Current State:** 3 files (chatgpt, claude, gemini)

**Action:**

1. **KEEP:** `claude-research.md` (referenced in CLAUDE.md)
2. **ARCHIVE:** Other two
```bash
mkdir -p /Users/jack/mag/dingo/ai-docs/archive/research/compiler/
mv /Users/jack/mag/dingo/ai-docs/research/compiler/chatgpt-research.md ai-docs/archive/research/compiler/
mv /Users/jack/mag/dingo/ai-docs/research/compiler/gemini_research.md ai-docs/archive/research/compiler/
```

**Optional Consolidation:** Merge all into `ARCHITECTURE.md`

**Files Reduced:** 3 → 1 (keep) + 2 (archive)

**Time Estimate:** 10 minutes

---

### Step 2.4: Fix Delegation Strategy Duplication

**Current State:** 2 identical files

**Action:**

1. **KEEP:** `ai-docs/research/delegation/delegation-strategy.md` (canonical)
2. **DELETE:** Root duplicate
```bash
rm /Users/jack/mag/dingo/ai-docs/delegation-strategy.md
```
3. **UPDATE CLAUDE.md:** Fix reference
```diff
- See `ai-docs/delegation-strategy.md`
+ See `ai-docs/research/delegation/delegation-strategy.md`
```

**Files Reduced:** 2 → 1

**Time Estimate:** 5 minutes

---

### Step 2.5: Update/Delete Delegation IMPLEMENTATION_STATUS.md

**File:** `ai-docs/research/delegation/IMPLEMENTATION_STATUS.md`

**Options:**
1. **UPDATE:** Add Phase V status
2. **DELETE:** Info already in CLAUDE.md

**Recommended:** DELETE (redundant with CLAUDE.md)

```bash
rm /Users/jack/mag/dingo/ai-docs/research/delegation/IMPLEMENTATION_STATUS.md
```

**Time Estimate:** 2 minutes

---

**Phase 2 Total Time:** ~32 minutes

**Phase 2 Result:**
- ✅ Enum naming: 5 → 1 file
- ✅ Go missing features: 5 files archived
- ✅ Compiler research: 3 → 1 file
- ✅ Delegation strategy: 2 → 1 file
- ✅ ~15 duplicate files eliminated

---

## Phase 3: Archive Old Sessions (Days 4-7)

**Priority:** 🟡 MEDIUM - Major space savings

### Step 3.1: Create Archive Structure

```bash
mkdir -p /Users/jack/mag/dingo/ai-docs/archive/{sessions,research,investigations,historical,architecture}
```

**Time Estimate:** 1 minute

---

### Step 3.2: Archive Old Session Folders (Nov 16-18)

**Current State:** 139 session folders, 75 from Nov 16-18

**Archive Criteria:**
- Sessions older than Nov 19 (today)
- Keep: 20251119-* folders
- Archive: 20251116-*, 20251117-*, 20251118-*

**Commands:**

```bash
# Archive Nov 16 sessions
mv /Users/jack/mag/dingo/ai-docs/sessions/20251116-* ai-docs/archive/sessions/

# Archive Nov 17 sessions
mv /Users/jack/mag/dingo/ai-docs/sessions/20251117-* ai-docs/archive/sessions/

# Archive Nov 18 sessions
mv /Users/jack/mag/dingo/ai-docs/sessions/20251118-* ai-docs/archive/sessions/

# Verify remaining
ls -1 /Users/jack/mag/dingo/ai-docs/sessions/ | wc -l
# Should be much smaller (~10-20 recent sessions)
```

**Files Archived:** ~2,250 files (75 sessions × ~30 files/session)

**Rationale:**
- Session logs are for active work
- Historical sessions preserved in archive
- CHANGELOG.md has implementation timeline

**Time Estimate:** 5 minutes (mostly mv time)

---

### Step 3.3: Archive Historical Root-Level Files

**Files to Archive:**

```bash
# Architecture decisions
mv /Users/jack/mag/dingo/ai-docs/architectural-comparison-option-b-vs-c.md ai-docs/archive/architecture/
mv /Users/jack/mag/dingo/ai-docs/package-scanning-architecture.md ai-docs/archive/architecture/
mv /Users/jack/mag/dingo/ai-docs/package-wide-function-detection-architecture.md ai-docs/archive/architecture/

# Historical summaries
mv /Users/jack/mag/dingo/ai-docs/dev-orchestrator-context-optimization.md ai-docs/archive/historical/
mv /Users/jack/mag/dingo/ai-docs/golang-developer-summary.md ai-docs/archive/historical/
mv /Users/jack/mag/dingo/ai-docs/golang-tester-project-state.md ai-docs/archive/historical/
mv /Users/jack/mag/dingo/ai-docs/parser-research.md ai-docs/archive/historical/

# Old reviews
mv /Users/jack/mag/dingo/ai-docs/reviews/phase4_2_review.md ai-docs/archive/reviews/

# Historical language UI decisions
mv /Users/jack/mag/dingo/ai-docs/language/UI_IMPLEMENTATION.md ai-docs/archive/historical/
```

**Files Archived:** 9 files

**Time Estimate:** 5 minutes

---

### Step 3.4: Archive Delegation Research

```bash
mv /Users/jack/mag/dingo/ai-docs/research/delegation/CLAUDISH_TIMEOUT_COMPLETE.md ai-docs/archive/research/delegation/
rm /Users/jack/mag/dingo/ai-docs/research/delegation/CLAUDISH_TIMEOUT_UPDATE.md  # Superseded
```

**Files Archived:** 1 file
**Files Deleted:** 1 file

**Time Estimate:** 2 minutes

---

**Phase 3 Total Time:** ~13 minutes (+ mv command execution time)

**Phase 3 Result:**
- ✅ ~2,250 session files archived (54% of all docs)
- ✅ 9 historical files archived
- ✅ Clean ai-docs/ structure remains
- ✅ All historical content preserved in archive/

---

## Post-Consolidation Verification

### Verification Checklist

```bash
# 1. Verify essential files exist
test -f /Users/jack/mag/dingo/README.md && echo "✅ README.md"
test -f /Users/jack/mag/dingo/CLAUDE.md && echo "✅ CLAUDE.md"
test -f /Users/jack/mag/dingo/CHANGELOG.md && echo "✅ CHANGELOG.md"
test -f /Users/jack/mag/dingo/features/INDEX.md && echo "✅ features/INDEX.md"

# 2. Verify features/INDEX.md updated
grep -q "✅ Implemented (Phase 2)" /Users/jack/mag/dingo/features/INDEX.md && echo "✅ INDEX.md updated"

# 3. Count remaining sessions
SESSIONS=$(ls -1 /Users/jack/mag/dingo/ai-docs/sessions/ | wc -l)
echo "Active sessions: $SESSIONS (should be < 20)"

# 4. Verify archive created
test -d /Users/jack/mag/dingo/ai-docs/archive/sessions && echo "✅ Archive structure exists"

# 5. Count archived sessions
ARCHIVED=$(ls -1d /Users/jack/mag/dingo/ai-docs/archive/sessions/*/ | wc -l)
echo "Archived sessions: $ARCHIVED (should be ~75)"

# 6. Verify no duplicates
! test -f /Users/jack/mag/dingo/ai-docs/delegation-strategy.md && echo "✅ No delegation duplicate"
! test -f /Users/jack/mag/dingo/ai-docs/enum-naming-convention-analysis.md && echo "✅ No enum duplicate"

# 7. Count essential ai-docs files
ESSENTIAL=$(find /Users/jack/mag/dingo/ai-docs -maxdepth 1 -type f -name "*.md" | wc -l)
echo "ai-docs root files: $ESSENTIAL (should be ~3)"

# 8. Verify deleted bug investigations
! test -f /Users/jack/mag/dingo/ai-docs/CRITICAL-2-FIX-SUMMARY.md && echo "✅ Critical bugs doc deleted"
! test -d /Users/jack/mag/dingo/ai-docs/lsp-parallel && echo "✅ LSP investigations deleted"
```

**Expected Output:**
```
✅ README.md
✅ CLAUDE.md
✅ CHANGELOG.md
✅ features/INDEX.md
✅ INDEX.md updated
Active sessions: 15 (should be < 20)
✅ Archive structure exists
Archived sessions: 75 (should be ~75)
✅ No delegation duplicate
✅ No enum duplicate
ai-docs root files: 3 (should be ~3)
✅ Critical bugs doc deleted
✅ LSP investigations deleted
```

---

## File Count Comparison

### Before Consolidation
```
Total markdown files: 1,167
├── Root: 3
├── .claude/: 12
├── features/: 19
├── ai-docs/: ~1,050+
│   ├── sessions/: ~4,170 (139 folders × 30 files)
│   ├── research/: 16
│   ├── investigations/: 26
│   └── root-level: 18
├── tests/golden/: 1
├── pkg/: 3
├── examples/: 4
└── editors/: 3
```

### After Consolidation
```
Total essential files: ~52
├── Root: 3
├── .claude/: 12 (unchanged)
├── features/: 19 (unchanged)
├── ai-docs/: ~8
│   ├── README.md
│   ├── ARCHITECTURE.md
│   ├── language/SYNTAX_DESIGN.md
│   ├── prompts/agent-anti-recursion-rules.md
│   ├── research/
│   │   ├── delegation/delegation-strategy.md
│   │   └── enum-naming-recommendations.md
│   └── archive/ (~1,100+ files preserved)
├── tests/golden/: 1
├── pkg/: 3
├── examples/: 4
└── editors/: 3

Total archived files: ~1,100+ (preserved, not deleted)
Total deleted files: ~50 (duplicates, resolved bugs)
```

**Reduction:** 1,167 → 52 essential files = **95.5% reduction**

---

## Implementation Timeline

### Week 1: Critical Fixes & Duplicates

**Day 1 (Today):**
- ✅ Phase 1: Critical fixes (45 min)
  - Fix features/INDEX.md
  - Establish source of truth
  - Delete misleading files

**Day 2:**
- ✅ Phase 2: Consolidate duplicates (30 min)
  - Enum naming
  - Go missing features
  - Compiler research
  - Delegation strategy

**Day 3:**
- ✅ Verification & testing (30 min)
  - Run verification checklist
  - Spot-check archived files
  - Update CLAUDE.md with completion status

### Week 2: Archive Sessions

**Day 4:**
- ✅ Phase 3: Archive structure & sessions (15 min)
  - Create archive directories
  - Move old sessions (Nov 16-18)

**Day 5:**
- ✅ Phase 3 continued: Archive historical files (10 min)
  - Move root-level historical docs
  - Archive old research

**Day 6:**
- ✅ Final verification (30 min)
  - Complete verification checklist
  - Document new structure
  - Update this plan with "COMPLETE" status

**Day 7:**
- ✅ Establish ongoing policies (30 min)
  - Set up weekly session archive reminder
  - Document retention policy in CLAUDE.md
  - Create session cleanup script (optional)

---

## Ongoing Maintenance Policy

### Weekly Tasks (5 minutes)

**Every Monday:**
```bash
# Archive sessions older than 7 days
CUTOFF_DATE=$(date -v-7d +%Y%m%d)
mv /Users/jack/mag/dingo/ai-docs/sessions/[date < $CUTOFF_DATE]-* ai-docs/archive/sessions/
```

**Automation Option:**
Create `/Users/jack/mag/dingo/scripts/archive-old-sessions.sh`:
```bash
#!/bin/bash
# Archive sessions older than 7 days

CUTOFF=$(date -v-7d +%Y%m%d)
SESSIONS_DIR="/Users/jack/mag/dingo/ai-docs/sessions"
ARCHIVE_DIR="/Users/jack/mag/dingo/ai-docs/archive/sessions"

for session in "$SESSIONS_DIR"/20*; do
  SESSION_DATE=$(basename "$session" | cut -d- -f1)
  if [[ "$SESSION_DATE" < "$CUTOFF" ]]; then
    echo "Archiving: $(basename "$session")"
    mv "$session" "$ARCHIVE_DIR/"
  fi
done

echo "✅ Session archive complete"
```

**Cron job (optional):**
```bash
# Every Monday at 9 AM
0 9 * * 1 /Users/jack/mag/dingo/scripts/archive-old-sessions.sh
```

---

### Monthly Tasks (15 minutes)

**First of month:**
1. Check for duplicate research files
2. Archive completed investigation folders
3. Update features/INDEX.md if new features implemented
4. Verify CLAUDE.md is current source of truth

---

### Per-Phase Tasks (30 minutes)

**At end of each development phase:**
1. Archive all phase-specific investigation docs
2. Update features/INDEX.md with new completions
3. Update CLAUDE.md current phase info
4. Create phase summary in CHANGELOG.md
5. Review ai-docs/ root for orphaned files

---

## Success Metrics

### Before Consolidation
- ❌ User confused (thought 0% complete)
- ❌ 1,167 files (impossible to navigate)
- ❌ 5 duplicate enum naming files
- ❌ 5 duplicate Go missing features files
- ❌ 26 LSP investigation files for resolved bugs
- ❌ No single source of truth

### After Consolidation
- ✅ User understands 58% complete
- ✅ 52 essential files (easy to navigate)
- ✅ 1 enum naming file (canonical)
- ✅ 0 duplicate Go missing features (archived)
- ✅ 0 LSP investigation files (deleted, in git history)
- ✅ CLAUDE.md is authoritative status source
- ✅ ~1,100 historical files preserved in archive/
- ✅ Clear retention policy prevents future sprawl

---

## Risk Mitigation

### What Could Go Wrong?

**Risk 1:** Delete file that's still referenced
- **Mitigation:** Only delete resolved bugs (fixes in git), archive everything else
- **Recovery:** All deletes are in git history

**Risk 2:** Archive file that's actively used
- **Mitigation:** Archive only sessions >7 days old, historical research
- **Recovery:** Git restore from archive/

**Risk 3:** Lose important information
- **Mitigation:** Archive first, delete only confirmed duplicates/resolved bugs
- **Verification:** All archived content accessible in ai-docs/archive/

**Risk 4:** Break references in CLAUDE.md
- **Mitigation:** Update CLAUDE.md references as part of Phase 1-2
- **Verification:** Grep for old paths after consolidation

---

## Rollback Plan

**If consolidation causes issues:**

```bash
# Restore from git
git checkout HEAD -- ai-docs/

# Or restore specific files
git checkout HEAD -- ai-docs/sessions/
git checkout HEAD -- ai-docs/research/
```

**All changes are in version control, 100% reversible.**

---

## Communication Plan

### Update CLAUDE.md

Add section documenting consolidation:
```markdown
## Documentation Consolidation (2025-11-19)

**Problem:** Documentation sprawl (1,167 files) caused confusion. User thought 0% complete when actually 58% done.

**Solution:** Consolidated to 52 essential files. Archived 1,100+ historical files. Deleted 50 duplicates/resolved bugs.

**Current Structure:**
- **Essential:** 52 files (this file, features/INDEX.md, .claude/, etc.)
- **Archived:** ai-docs/archive/ (sessions, research, investigations)
- **Retention Policy:** Archive sessions >7 days old

**Source of Truth:** This file (CLAUDE.md) is canonical for current status. features/INDEX.md for feature status.
```

### Update features/INDEX.md

Add note at top:
```markdown
> **Documentation Update (2025-11-19):** This file was updated to reflect actual implementation status.
> Previously showed all features "Not Started" when 11 were complete. Now accurate: 58% done (11/19 features).
```

---

## Next Steps After Consolidation

1. **Communicate to user:**
   - "Documentation cleaned up: 1,167 → 52 essential files"
   - "features/INDEX.md now shows accurate 58% completion"
   - "All historical content preserved in archive/"

2. **Establish as standard:**
   - Update agent instructions to reference new structure
   - Add retention policy to CLAUDE.md
   - Document archive structure in ai-docs/archive/README.md

3. **Prevent future sprawl:**
   - Weekly session archiving
   - One file per topic rule
   - Delete resolved investigations immediately
   - Consolidate research when feature implemented

---

**Next File:** 04-archive-commands.md (Exact bash commands to execute this plan)
