# Delegation Strategy & Context Economy

## Core Principle: Main Chat = Orchestration Only

The main conversation context should be **minimal and lean**, containing ONLY:
- High-level decisions and next steps
- Brief agent summaries (2-5 sentences max)
- User interactions and approvals
- File paths (NOT file contents)

**All detailed work happens in agent threads and files.**

## The Problem We're Solving

**BAD (Context Bloat)**:
```
Main Chat:
User: "Implement feature X"
Assistant: [Reads 5 files, 500 lines total]
Assistant: [Pastes full implementation plan, 200 lines]
Assistant: [Shows full code review, 150 lines]
Assistant: [Displays all test results, 100 lines]
Result: 950 lines in main context, can't remember earlier decisions
```

**GOOD (Context Economy)**:
```
Main Chat:
User: "Implement feature X"
Assistant: "Delegating to golang-developer agent..."
Agent returns: "Feature X implemented. 3 files modified. Summary: ai-docs/session-123/summary.txt"
Assistant: "Implementation complete. Ready for review?"
Result: <20 lines in main context, crisp decision-making
```

## Delegation Pattern: The Three-Layer Architecture

```
┌─────────────────────────────────────────────────────────┐
│ Layer 1: MAIN CHAT (Orchestrator)                      │
│ Role: Strategy, decisions, user interaction            │
│ Context: Minimal (summaries only)                      │
│ Output: Next action, delegation instructions           │
└─────────────────────────────────────────────────────────┘
                           ↓ Delegates via Task tool
┌─────────────────────────────────────────────────────────┐
│ Layer 2: AGENTS (Specialized Workers)                  │
│ Role: Deep investigation, implementation, analysis     │
│ Context: Full access to codebase                       │
│ Output: Files + Brief summary (3-5 sentences)          │
└─────────────────────────────────────────────────────────┘
                           ↓ Writes to
┌─────────────────────────────────────────────────────────┐
│ Layer 3: FILES (Persistent Storage)                    │
│ Role: Detailed reports, code, analysis                 │
│ Context: Complete implementation details               │
│ Output: Referenced by path, read only when needed      │
└─────────────────────────────────────────────────────────┘
```

## Communication Protocol

### Main Chat → Agent (Delegation)

**Template**:
```
Use Task tool with [agent-name]:

Task: [Specific, actionable task]

Input Files:
- [file1]
- [file2]

Your Job:
1. [Action 1]
2. [Action 2]
3. Write detailed results to: [output-path]

Return to Main Chat:
ONLY a 2-5 sentence summary in this format:

# [Task Name] Complete

Status: [Success/Partial/Failed]
Key Finding: [One-liner]
Changed: [N] files
Details: [output-path]
```

### Agent → Main Chat (Return)

**Agents MUST return concise summaries:**
```
# Error Propagation Implementation Complete

Status: Success
Implemented: ? operator in 3 files (preprocessor, transformer, tests)
Tests: 15/15 passing
Details: ai-docs/sessions/20251118-143022/implementation-summary.md
```

**Agents MUST NOT return:**
- ❌ Full implementation code
- ❌ Complete test output
- ❌ Detailed file listings
- ❌ Long explanations (save for files)

### Main Chat Uses Summary

**Main chat reads agent summary and decides:**
- ✅ Success? → Move to next phase
- ⚠️ Partial? → Ask user for guidance
- ❌ Failed? → Investigate or retry

**Main chat does NOT:**
- ❌ Read full implementation files (unless needed for user presentation)
- ❌ Parse detailed logs
- ❌ Store large data in context

## Agent Self-Awareness Rules (Anti-Recursion)

**CRITICAL FOR ALL AGENTS:**

### Rule 1: Know Thyself

Every agent MUST be aware of its own type:
- If you are `golang-developer`, you cannot delegate to `golang-developer`
- If you are `astro-developer`, you cannot delegate to `astro-developer`
- If you are `code-reviewer`, you cannot delegate to `code-reviewer`
- If you are `golang-tester`, you cannot delegate to `golang-tester`
- If you are `golang-architect`, you cannot delegate to `golang-architect`
- If you are `astro-reviewer`, you cannot delegate to `astro-reviewer`

**Why:** You ARE the specialized agent. Delegating to yourself causes recursion and failures.

### Rule 2: Delegation Decision Tree

```
Before using Task tool, ask:
│
├─ What is my agent type?
│  └─ I am: [agent-name]
│
├─ What agent type does this task need?
│  ├─ Same as me → ❌ DO NOT delegate. Implement directly.
│  └─ Different → ✅ CAN delegate to that different agent
│
└─ Why do I want to delegate?
   ├─ "To save context" → ❌ WRONG REASON. Just do the work.
   ├─ "Instructions say to" → ❌ Those are for CALLERS, not you.
   └─ "Need different expertise" → ✅ OK if it's a different agent type.
```

### Rule 3: Proxy Mode Is Not Self-Delegation

**Proxy Mode Means:**
- Using `claudish` to consult external models (Grok, Gemini, Codex)
- Getting suggestions/implementations from those models
- Implementing their suggestions yourself

**Proxy Mode Does NOT Mean:**
- Using Task tool to invoke yourself
- Creating another instance of your own agent
- Delegating work you should do directly

### Rule 4: Instructions Are Context-Dependent

When agents read their prompts and see:
- "Use the Task tool to invoke the golang-developer agent"
- "Delegate to astro-developer for implementation"

**Understand:** These instructions are for MAIN CHAT and EXTERNAL MODELS to use when calling the agent.

**NOT** for the agent to use to call itself.

The agent is the **destination** of those calls, not the **source**.

### Rule 5: When In Doubt, Implement

If an agent is uncertain whether to delegate:
1. Check if delegating to own agent type → If yes, DON'T
2. Check if it has the expertise to implement → If yes, DO IT
3. Check if trying to save context → NOT A VALID REASON

**Default action for agents: Implement directly.**

### Rule 6: External Model Invocation

When a specialized agent (golang-architect, astro-developer, etc.) is asked to invoke an external model:

✅ **Correct approach:**
- Use Bash tool to invoke claudish
- Read input prompt from file
- Save full response to file
- Return brief summary (MAX 5 sentences)

❌ **Incorrect approaches:**
- Delegating to another agent to invoke claudish
- Trying to invoke claudish via Task tool
- Returning full analysis in response (context bloat)

**Example (golang-architect agent)**:
```bash
# Read prompt
cat /path/to/investigation-prompt.md | \
  claudish --model openai/gpt-5.1-codex > \
  /path/to/output/gpt-5.1-codex-analysis.md

# Return brief summary only
```

### Examples

**✅ CORRECT Delegation (Different Agent Types):**
- `golang-developer` delegates to `golang-tester` (different agent ✅)
- `astro-developer` delegates to `astro-reviewer` (different agent ✅)
- `golang-developer` delegates to `Explore` (different agent ✅)
- `golang-architect` delegates to `golang-developer` (different agent ✅)

**❌ WRONG Delegation (Recursion - Same Agent Type):**
- `golang-developer` delegates to `golang-developer` (same agent ❌)
- `astro-developer` delegates to `astro-developer` (same agent ❌)
- `code-reviewer` delegates to `code-reviewer` (same agent ❌)

## When to Delegate vs. Handle Directly

### ✅ DELEGATE (Use Skills or Task Tool)

**Use Skills for Common Patterns**:
- Multi-model consultation → `multi-model-consult` skill
- Codebase investigation → `investigate` skill
- Feature implementation → `implement` skill
- Testing tasks → `test` skill

**Use Task Tool Directly** (without skill):
- One-off agent tasks
- Simple code review
- Quick analysis
- Tasks not covered by skills

### ⚠️ HANDLE DIRECTLY (Main Chat)

**Only handle these yourself**:
- User interaction (questions, approvals, summaries)
- Single file read (known path)
- Single line fix
- Git status check
- Coordination (launching agents, deciding next phase)

**Rule of thumb**: If it takes >3 steps or >2 files → Delegate!

## File-Based Communication Patterns

### Session Folders (Recommended for Workflows)

For orchestrated workflows (like `/dev`), use session folders:

```
ai-docs/sessions/YYYYMMDD-HHMMSS/
├── 01-planning/
│   ├── requirements.md       # Agent writes
│   ├── plan.md              # Agent writes
│   └── plan-summary.txt     # Agent returns this to main chat
├── 02-implementation/
│   ├── changes.md           # Agent writes
│   └── summary.txt          # Agent returns this to main chat
├── 03-review/
│   ├── review-full.md       # Agent writes
│   └── summary.txt          # Agent returns this to main chat
└── session-state.json       # Orchestrator writes
```

**Orchestrator** (main chat):
- Reads ONLY: `*-summary.txt` files and `session-state.json`
- Writes: `session-state.json` (tracks workflow state)
- Presents summaries to user

**Agents**:
- Write: Full detailed reports to `.md` files
- Write: Summary to `*-summary.txt`
- Return: "Summary written to: [path]"

### Quick Tasks (Ad-hoc Delegation)

For one-off tasks, use simpler paths:

```
Main Chat: "Understand how Result[T,E] works"

Delegates to golang-developer:
- Investigates codebase
- Writes: ai-docs/analysis/result-type-analysis.md
- Returns: "Result[T,E] is implemented via AST transformation.
           Uses IIFE pattern for constructors.
           Details: ai-docs/analysis/result-type-analysis.md"

Main Chat:
- Receives summary
- Decides: Good enough? Or need more detail?
- If need detail: Reads the file
- Otherwise: Continues with next task
```

## Parallel Execution

**When agents can run in parallel**, launch ALL of them in a **single message**:

```
I'm launching 3 agents in parallel:

[Task tool call 1: golang-developer - Implement feature A]
[Task tool call 2: golang-developer - Implement feature B]
[Task tool call 3: golang-tester - Create tests]

(All 3 tool calls in ONE message)
```

**Benefits**:
- 3x speedup (tasks run simultaneously)
- All summaries come back together
- Can aggregate results efficiently

**Rules**:
- ✅ Parallel: Independent tasks (different files, separate features)
- ❌ Sequential: Dependencies (tests need implementation first)

## Example: Full Workflow with Delegation

**User Request**: "Implement lambda syntax for Dingo"

```
┌─────────────────────────────────────────────────────────┐
│ MAIN CHAT (Orchestrator)                                │
└─────────────────────────────────────────────────────────┘

User: "Implement lambda syntax for Dingo"

Main Chat:
1. Creates session: ai-docs/sessions/20251118-150000/
2. Delegates to golang-architect: "Plan lambda implementation"

┌─────────────────────────────────────────────────────────┐
│ GOLANG-ARCHITECT AGENT                                  │
└─────────────────────────────────────────────────────────┘

Agent:
- Reads features/lambdas.md
- Designs implementation approach
- Writes: 01-planning/plan.md (5 pages, detailed)
- Writes: 01-planning/summary.txt (3 sentences)
- Returns: "Lambda plan complete. 3-phase approach: preprocessor,
           parser, codegen. Details: 01-planning/plan.md"

┌─────────────────────────────────────────────────────────┐
│ MAIN CHAT (Orchestrator)                                │
└─────────────────────────────────────────────────────────┘

Main Chat:
- Receives summary (3 sentences)
- Reads plan.md for user presentation
- Shows plan to user
- User approves

Main Chat:
3. Launches 2 parallel golang-developer agents:
   - Agent A: Implement preprocessor
   - Agent B: Implement parser

┌─────────────────────────────────────────────────────────┐
│ GOLANG-DEVELOPER AGENTS (Parallel)                      │
└─────────────────────────────────────────────────────────┘

Agent A:
- Implements preprocessor in pkg/generator/preprocessor/
- Writes: 02-implementation/preprocessor-changes.md
- Returns: "Preprocessor complete. 2 files modified.
           Details: 02-implementation/preprocessor-changes.md"

Agent B (simultaneously):
- Implements parser patterns
- Writes: 02-implementation/parser-changes.md
- Returns: "Parser complete. 3 files modified.
           Details: 02-implementation/parser-changes.md"

┌─────────────────────────────────────────────────────────┐
│ MAIN CHAT (Orchestrator)                                │
└─────────────────────────────────────────────────────────┘

Main Chat:
- Receives both summaries
- Aggregates: "5 files modified total"
- Announces to user: "Implementation phase complete"

Main Chat:
4. Launches golang-tester: "Create tests for lambda syntax"

┌─────────────────────────────────────────────────────────┐
│ GOLANG-TESTER AGENT                                      │
└─────────────────────────────────────────────────────────┘

Agent:
- Creates golden tests
- Runs test suite
- Writes: 04-testing/test-results.md (full output, 200 lines)
- Writes: 04-testing/summary.txt (pass/fail counts)
- Returns: "Tests complete. 45/45 passing.
           Details: 04-testing/test-results.md"

┌─────────────────────────────────────────────────────────┐
│ MAIN CHAT (Orchestrator)                                │
└─────────────────────────────────────────────────────────┘

Main Chat:
- Receives summary: "45/45 passing"
- Announces to user: "Lambda implementation complete! All tests pass."
- Total context used: <50 lines (not 500+ lines)
```

## Metrics: Context Savings

**Traditional Approach** (no delegation):
- User request: 1 line
- Read 10 files: 500 lines
- Implementation code shown: 200 lines
- Test output shown: 150 lines
- Review discussion: 100 lines
- **Total: ~950 lines in main context** ❌

**Delegation Approach**:
- User request: 1 line
- Agent delegation: 3 lines × 4 agents = 12 lines
- Agent summaries: 3 lines × 4 agents = 12 lines
- Orchestrator decisions: 10 lines
- User announcements: 5 lines
- **Total: ~40 lines in main context** ✅

**Result**: **23x reduction in context usage** while achieving the same outcome!

## Key Rules for All Participants

### Main Chat (Orchestrator) Rules:
1. ✅ Delegate complex tasks to agents
2. ✅ Read only summaries from agents
3. ✅ Read full files only when presenting to user
4. ✅ Launch parallel agents in single message
5. ✅ Keep context minimal (summaries + decisions)
6. ❌ Don't read full implementation files into context
7. ❌ Don't store detailed data in conversation
8. ❌ Don't do deep analysis yourself (delegate it)

### Agent Rules:
1. ✅ Do deep investigation and implementation
2. ✅ Write ALL details to files
3. ✅ Return ONLY 2-5 sentence summary
4. ✅ Include file paths in summary
5. ✅ Use session folders for workflow tasks
6. ❌ Don't return full code in response
7. ❌ Don't return detailed logs in response
8. ❌ Don't return multi-page explanations in response

### User Expectations:
- Main chat shows high-level progress
- Detailed results available in files
- Can request to see specific files
- Quick, clear decision points
- No context overwhelm

## Quick Delegation Templates

**For common patterns**, use skills (investigate, implement, test, multi-model-consult).

**For ad-hoc tasks**, delegate directly with Task tool:

**Basic Template**:
```
Task tool → [agent-type]:

[Task description]

Your Tasks:
1. [Action 1]
2. [Action 2]
3. Write detailed results to: [output-path]

Return to Main Chat (MAX 5 sentences):
[Brief summary format]
Details: [output-path]

DO NOT return full details in response.
```

**Agent MUST**:
- Write ALL details to files
- Return ONLY 2-5 sentence summary
- Include file path in response

See skills (`investigate.md`, `implement.md`, `test.md`) for detailed templates.

## Summary: Delegation Strategy Benefits

✅ **Context Economy**: 10-20x reduction in main chat context usage
✅ **Clarity**: Clear separation between orchestration and execution
✅ **Parallel Speedup**: 2-4x faster via concurrent agent execution
✅ **Persistence**: All work saved to files, nothing lost
✅ **Scalability**: Can handle large, complex multi-phase projects
✅ **Focus**: Main chat stays focused on decisions, not details

**Remember**:
- **Main chat = Strategy and decisions**
- **Agents = Investigation and execution**
- **Files = Detailed persistent storage**

This is the key to handling complex projects without context overload!
